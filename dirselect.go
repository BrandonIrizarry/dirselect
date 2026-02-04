package dirselect

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Various state variables used to manage the model. These could in
// fact be fields within [Model] itself, but having them as
// package-local state variables suffices.
var (
	// The file used for logging. This file is meant to be closed just
	// before we send a [tea.Quit] message in [Model.Update].
	logFile *os.File

	// The quitting flag signals whether we're about to send
	// [tea.Quit], so that we can return an empty string inside
	// [Model.View]. This prevents a stale, garbled UI display
	// from lingering on after we exit the application hosting the
	// model.
	quitting bool

	// homeDir is the user's home directory, stored for
	// allowing jumps back to it.
	homeDir string

	// currentDir is the path of the currently explored
	// directory. This should always be an absolute path. In
	// practice, this should always be the case since the
	// top-level always initializes it with the result of
	// [os.UserHomeDir].
	currentDir string

	// viewMin and viewMax are the bounds of the current viewable
	// portion of the directory picker [Model].
	viewMin, viewMax int

	// lineNumber is the zero-indexed line number pointed to by
	// the cursor.
	lineNumber int

	// dirListing is the list of directories inside the currently
	// explored directory. It always has at least one entry, '..',
	// allowing the user to navigate to the parent directory. This
	// is partly to avoid panics involving empty slices in the
	// case of otherwise empty directories.
	dirListing []string

	// KeyMap defines key bindings for each user action.
	keyMap = struct {
		down         key.Binding
		up           key.Binding
		beginning    key.Binding
		end          key.Binding
		back         key.Binding
		explore      key.Binding
		toggleSelect key.Binding
		jump         key.Binding
		jumpToHome   key.Binding
		quit         key.Binding
	}{
		up:           key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k/↑/ctrl+p", "previous line")),
		down:         key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j/↓/ctrl+n", "next line")),
		beginning:    key.NewBinding(key.WithKeys("g", "alt+<"), key.WithHelp("g/M-<", "go to top of listing")),
		end:          key.NewBinding(key.WithKeys("G", "alt+>"), key.WithHelp("G/M->", "go to bottom of listing")),
		back:         key.NewBinding(key.WithKeys("h", "left", "ctrl+b"), key.WithHelp("h/←/ctrl+b", "go to parent directory")),
		explore:      key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("l/→/enter", "explore this directory")),
		jump:         key.NewBinding(key.WithKeys("0", "1", "2", "3", "4", "5", "6", "7", "8", "9"), key.WithHelp("0-9", "jump to selection")),
		jumpToHome:   key.NewBinding(key.WithKeys("~"), key.WithHelp("~", "jump back to home directory")),
		toggleSelect: key.NewBinding(key.WithKeys(" "), key.WithHelp("spacebar", "toggle selection")),
		quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit")),
	}
)

func New() (Model, error) {
	// Set up logging first. We'll close the file just before
	// returning the [tea.Quit] command. We can do this because we
	// store it in the model's state.
	logFile, err := os.OpenFile("debug.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return Model{}, fmt.Errorf("couldn't create debug.log: %w", err)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Now onto the business of the model itself.
	homeDir, err = os.UserHomeDir()
	if err != nil {
		return Model{}, fmt.Errorf("cannot create dirselect widget: %w", err)
	}

	// Unforunately we can't do this when first declaring
	// currentDir, so we mustn't forget to do it here.
	currentDir = homeDir

	return Model{
		SelectedDirs: make([]string, 0),
	}, nil
}

// readDir returns a [tea.Cmd] that tells the UI to update with the
// list of directories corresponding to a users selection ('explore',
// 'back', etc.)
//
// Its sole purpose is to supply the path argument to the underlying
// closure, which is then returned as the actual command.
func (m Model) readDir(path, startEntry string) tea.Cmd {
	// All directory listings start with an entry corresponding to
	// the parent directory; see [Model.dirListing].
	dirs := []string{".."}

	return func() tea.Msg {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, d := range dirEntries {
			if d.IsDir() {
				dirs = append(dirs, d.Name())
			}
		}

		return readDirMsg{path: path, entries: dirs, startDir: startEntry}
	}
}

// dirAtPoint is sugar for returning the directory that the cursor is
// currently resting on. The directory is returned as an absolute
// path, per how [Model.currentDir] is always set.
//
// Note that we never edit the entries themselves, so it's OK for us
// to only have a getter method for this field.
func (m Model) dirAtPoint() string {
	return filepath.Join(currentDir, dirListing[lineNumber])
}

// back adjusts all the state necessary to effect a "cd .." operation.
func (m Model) back() (tea.Model, tea.Cmd) {
	path := filepath.Dir(currentDir)
	startDir := filepath.Base(currentDir)

	return m, m.readDir(path, startDir)
}

func (m Model) explore() (tea.Model, tea.Cmd) {
	path := m.dirAtPoint()
	startDir := ".."

	return m, m.readDir(path, startDir)
}

func (m *Model) scrollDown(times int) {
	log.Printf("times: %d; line number before scroll-down code: %d", times, lineNumber)
	for range times {
		lineNumber++
		if lineNumber > len(dirListing)-1 {
			lineNumber = len(dirListing) - 1
		}

		if viewMax < len(dirListing)-1 && lineNumber > (viewMax+viewMin)/2 {
			viewMin++
			viewMax++
		}
	}
	log.Printf("line number after scroll-down code: %d", lineNumber)
}

func (m *Model) scrollUp(times int) {
	for range times {
		lineNumber--
		if lineNumber < 0 {
			lineNumber = 0
		}

		if viewMin > 0 && lineNumber < (viewMax+viewMin)/2 {
			viewMin--
			viewMax--
		}
	}
}

func (m Model) Init() tea.Cmd {
	log.Printf("Starting by reading %s", currentDir)
	return m.readDir(currentDir, "..")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case readDirMsg:
		// FIXME: this is very sensitive to state; I have to
		// make sure dirListing is set before scrollDown is
		// called, since it references this state being
		// mutated here!
		dirListing = msg.entries

		viewMin = 0
		viewMax = min(10, len(dirListing)) - 1
		lineNumber = 0

		found := false
		for i, e := range msg.entries {
			if e == msg.startDir {
				log.Printf("found start dir %s at pos %d", e, i)
				m.scrollDown(i)
				found = true
				break
			}
		}

		if !found {
			log.Printf("Couldn't find pointed-to entry: %s", msg.startDir)
		} else {
			log.Printf("line number: %d", lineNumber)
		}

		currentDir = msg.path

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyMap.back):
			if currentDir == homeDir {
				break
			}

			return m.back()

		case key.Matches(msg, keyMap.beginning):
			m.scrollUp(100)

		case key.Matches(msg, keyMap.down):
			m.scrollDown(1)

		case key.Matches(msg, keyMap.end):
			m.scrollDown(100)

		case key.Matches(msg, keyMap.explore):
			if lineNumber == 0 && currentDir == homeDir {
				break
			}

			if lineNumber == 0 {
				return m.back()
			}

			return m.explore()

		case key.Matches(msg, keyMap.jump):
			index, err := strconv.Atoi(msg.String())
			if err != nil {
				panic("FIXME: save error to m.err")
			}

			// FIXME: 0-9 is the allowed range of jump
			// indices, and therefore 10 should be the
			// maximum number of selectable directories.
			if index >= len(m.SelectedDirs) || index > 9 {
				return m, nil
			}

			selection := m.SelectedDirs[index]

			// Note that the selection's parent directory
			// is the actual destination of the jump.
			path := filepath.Dir(selection)
			startDir := filepath.Base(selection)

			return m, m.readDir(path, startDir)

		case key.Matches(msg, keyMap.jumpToHome):
			return m, m.readDir(homeDir, "..")

		case key.Matches(msg, keyMap.toggleSelect):
			// Disable selection of the ".." entry.
			if lineNumber == 0 {
				break
			}

			dir := m.dirAtPoint()

			log.Printf("Candidate for toggling: %s", dir)

			// Toggle the presence of the directory in the
			// map.
			if pos := slices.Index(m.SelectedDirs, dir); pos != -1 {
				log.Printf("Removed selected dir at pos %d", pos)
				m.SelectedDirs = slices.Delete(m.SelectedDirs, pos, pos+1)
			} else {
				log.Printf("Added %s to selected dirs", dir)

				// If there are already 10 selected directories, do nothing.
				if len(m.SelectedDirs) == 10 {
					break
				}

				m.SelectedDirs = append(m.SelectedDirs, dir)
			}

		case key.Matches(msg, keyMap.up):
			m.scrollUp(1)

		case key.Matches(msg, keyMap.quit):
			logFile.Close()
			quitting = true
			return m, tea.Quit
		}
	}

	log.Printf("Uncaught message: %v", msg)
	return m, nil
}

func (m Model) View() string {
	// Don't render anything in thise case; see [quitting].
	if quitting {
		return ""
	}

	// Some local declarations to make things more readable.
	var (
		view        strings.Builder
		borderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 5)
		entryStyle = lipgloss.NewStyle().
				MarginLeft(10)
		arrowStyle = entryStyle.Width(30).Align(lipgloss.Center)
		upArrow    = arrowStyle.Render("↑")
		downArrow  = arrowStyle.Render("↓")
	)

	// Display the "jump list."
	for i, s := range m.SelectedDirs {
		fmt.Fprintf(&view, "%d: %s\n", i, s)
	}

	// Display an "↑" to show there are directories hidden from
	// display above the currently viewable region of the UI. Note
	// that we otherwise render an empty string with the same
	// [lipgloss.Style], to conserve the spacing taken up by the
	// arrow.
	if viewMin > 0 {
		view.WriteString(upArrow)
	} else {
		view.WriteString(arrowStyle.Render(""))
	}

	// Add a newline before listing entries, so that introducing
	// "↑" doesn't make the view look janky.
	view.WriteString("\n")

	for i, d := range dirListing {
		// If d is one of the selected directories, then
		// display a checkmark; else leave a space.
		mark := " "
		if slices.Contains(m.SelectedDirs, filepath.Join(currentDir, d)) {
			mark = "✓"
		}

		var entry string
		emphasized := lipgloss.NewStyle().Underline(true).Render(d)

		switch {
		case i == 0:
			// See the default case.
			if lineNumber == 0 {
				entry = fmt.Sprintf("→     %s", emphasized)
			} else {
				entry = fmt.Sprintf("      %s", d)
			}

		case i < viewMin || i > viewMax:
			// Enforce that only the current viewport height be
			// displayed.
			continue

		default:
			// Inform the display which line is currently being
			// pointed at by the cursor.
			if i == lineNumber {
				entry = fmt.Sprintf("→ [%s] %s", mark, emphasized)
			} else {
				entry = fmt.Sprintf("  [%s] %s", mark, d)
			}
		}

		// Here we're careful not to render the newline
		// string, since this causes display problems.
		view.WriteString(entryStyle.Render(entry) + "\n")
	}

	// See remarks above pertaining to displaying "↑".
	if viewMax < len(dirListing)-1 {
		view.WriteString(downArrow)
	} else {
		view.WriteString(arrowStyle.Render(""))
	}

	return borderStyle.Render(view.String())
}
