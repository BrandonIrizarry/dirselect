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

	showHidden bool

	// KeyMap defines key bindings for each user action.
	keyMap = map[string]key.Binding{
		"up":           key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k/↑/ctrl+p", "previous line")),
		"down":         key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j/↓/ctrl+n", "next line")),
		"beginning":    key.NewBinding(key.WithKeys("g", "alt+<"), key.WithHelp("g/M-<", "go to top of listing")),
		"end":          key.NewBinding(key.WithKeys("G", "alt+>"), key.WithHelp("G/M->", "go to bottom of listing")),
		"back":         key.NewBinding(key.WithKeys("h", "left", "ctrl+b"), key.WithHelp("h/←/ctrl+b", "go to parent directory")),
		"explore":      key.NewBinding(key.WithKeys("l", "right", "enter", "ctrl+f"), key.WithHelp("l/→/enter", "explore this directory")),
		"jump":         key.NewBinding(key.WithKeys("0", "1", "2", "3", "4", "5", "6", "7", "8", "9"), key.WithHelp("0-9", "jump to selection")),
		"jumpToHome":   key.NewBinding(key.WithKeys("~"), key.WithHelp("~", "jump to home directory")),
		"toggleSelect": key.NewBinding(key.WithKeys(" "), key.WithHelp("spacebar", "toggle selection")),
		"toggleHidden": key.NewBinding(key.WithKeys("."), key.WithHelp(".", "hide/show hidden directories")),
		"quit":         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit")),
	}
)

const (
	// maxViewHeight is the number of entries visible through the viewport
	// at any given moment,
	maxViewHeight = 15

	// maxSelections is the maximum size [Model.SelectedDirs], set
	// as the number of bindings associated with the quick jumps
	// to those directories.
	maxSelections = 10
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
				reportsAsHidden, err := IsHidden(d.Name())
				if err != nil {
					panic(err)
				}

				if reportsAsHidden && !showHidden {
					continue
				}

				dirs = append(dirs, d.Name())
			}
		}

		return readDirMsg{currentDir: path, entries: dirs, startDir: startEntry}
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
	// Conveniently, both filepath.Dir("/") and filepath.Base("/")
	// are "/", so when moving back when already at the top, we
	// simply end up rereading the top, looking for "/". This ends
	// up failing, but gracefully. See also [Model.Update], under
	// the [readDirMsg] switch clause.
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
		log.Printf("readDirMsg: %v", msg)

		// NOTE: this is very sensitive to state; I have to
		// make sure dirListing is set before scrollDown is
		// called, since it references this state being
		// mutated here!
		dirListing = msg.entries

		viewMin = 0
		viewMax = min(maxViewHeight, len(dirListing)) - 1
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

		// It could be the case that, while the cursor is on a
		// hidden directory, the user presses the key that
		// activates the binding in [keyMap] to toggle hidden
		// directories. In that case, the directory is reread,
		// but the [readDirMsg.startDir] no longer
		// exists. That's OK, since the default settings for
		// viewMin, viewMax, and lineNumber (see above) take
		// care of this: we simply start on "..".
		//
		// This also comes in handy when moving up when
		// visiting the root directory, where readDirMsg is
		// effectively ("/", "/"). Since "/" isn't an an entry
		// of itself, the loop breaks and we start on ".."
		// (see [Model.back]).
		if !found {
			log.Printf("Couldn't find pointed-to entry: %s", msg.startDir)
		} else {
			log.Printf("line number: %d", lineNumber)
		}

		currentDir = msg.currentDir

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyMap["back"]):
			return m.back()

		case key.Matches(msg, keyMap["beginning"]):
			m.scrollUp(len(dirListing) - 1)

		case key.Matches(msg, keyMap["down"]):
			m.scrollDown(1)

		case key.Matches(msg, keyMap["end"]):
			m.scrollDown(len(dirListing) - 1)

		case key.Matches(msg, keyMap["explore"]):
			if lineNumber == 0 {
				return m.back()
			}

			return m.explore()

		case key.Matches(msg, keyMap["jump"]):
			index, err := strconv.Atoi(msg.String())
			if err != nil {
				panic("Fatal: can't convert jumplist index to int")
			}

			// 0-9 is the allowed range of jump indices;
			// see [maxSelections].
			if index >= len(m.SelectedDirs) || index >= maxSelections {
				return m, nil
			}

			selection := m.SelectedDirs[index]

			// Note that the selection's parent directory
			// is the actual destination of the jump.
			path := filepath.Dir(selection)
			startDir := filepath.Base(selection)

			return m, m.readDir(path, startDir)

		case key.Matches(msg, keyMap["jumpToHome"]):
			return m, m.readDir(homeDir, "..")

		case key.Matches(msg, keyMap["toggleHidden"]):
			showHidden = !showHidden
			return m, m.readDir(currentDir, dirListing[lineNumber])

		case key.Matches(msg, keyMap["toggleSelect"]):
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

				// If there are already [maxSelections] selected directories, do nothing.
				if len(m.SelectedDirs) == maxSelections {
					break
				}

				m.SelectedDirs = append(m.SelectedDirs, dir)
			}

		case key.Matches(msg, keyMap["up"]):
			m.scrollUp(1)

		case key.Matches(msg, keyMap["quit"]):
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
				Padding(0, 5).
				Height(len(keyMap) + 4)
		entryStyle = lipgloss.NewStyle().
				MarginLeft(10)
		arrowStyle = entryStyle.Align(lipgloss.Center)
		upArrow    = arrowStyle.Render("   ↑")
		downArrow  = arrowStyle.Render("   ↓")
	)

	// Display an "↑" to show there are directories hidden from
	// display above the currently viewable region of the UI.
	if viewMin > 0 {
		view.WriteString(upArrow)
	}

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

		case i <= viewMin || i > viewMax:
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
		view.WriteString("\n" + entryStyle.Render(entry))
	}

	// See remarks above pertaining to displaying "↑".
	if viewMax < len(dirListing)-1 {
		view.WriteString("\n" + downArrow)
	}

	// Display the keybinding help text.
	var viewHelp strings.Builder
	type binding struct {
		actionName string
		key        key.Binding
	}
	var buf []binding

	for actionName, key := range keyMap {
		buf = append(buf, binding{actionName, key})
	}

	slices.SortFunc(buf, func(b1, b2 binding) int {
		desc1 := b1.key.Help().Desc
		desc2 := b2.key.Help().Desc

		if desc1 < desc2 {
			return -1
		}

		return 1
	})

	for _, b := range buf {
		help := b.key.Help()
		fmt.Fprintf(&viewHelp, "\n%s: %s", help.Desc, help.Key)
	}

	// NOTE: borrow viewHelp for now for displaying the jump
	// list.
	//
	// Display the "jump list."
	viewHelp.WriteString("\n\nJump list:")
	for i, s := range m.SelectedDirs {
		baseName := filepath.Base(s)
		fmt.Fprintf(&viewHelp, "\n%d: %s", i, baseName)
	}

	right := view.String()
	left := viewHelp.String()

	return borderStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, left, right))
}
