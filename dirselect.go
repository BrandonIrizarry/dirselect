package dirselect

import (
	"errors"
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

	// The depth of the home directory in the filesystem. I would
	// assume this should be 2 in all cases (e.g. "/home/user"),
	// but I could be wrong.
	//
	// This helps for restoring a usable [lineNumberStack] when
	// jumping to a directory other than the home directory.
	homeDirDepth int
)

var ErrEmptyStack = errors.New("empty lineNumberStack")

// The stack saves our place for when we want to move back up
// directories again ("breadcrumbs"). It also implicitly keeps track
// of how deep we've traversed from the user's home directory, and for
// example prevents any attempt to explore above it.
type stack struct {
	push  func(int)
	pop   func() (int, error)
	depth func() int
	reset func()
}

func newStack() stack {
	slice := make([]int, 0)
	return stack{
		push: func(i int) {
			slice = append(slice, i)
		},
		pop: func() (int, error) {
			if len(slice) == 0 {
				return 0, ErrEmptyStack
			}

			res := slice[len(slice)-1]
			slice = slice[:len(slice)-1]
			return res, nil
		},
		depth: func() int {
			return len(slice)
		},
		reset: func() {
			slice = make([]int, 0)
		},
	}
}

var lineNumberStack = newStack()

// FIXME: inline this into explore().
func (m *Model) saveLineNumber() {
	lineNumberStack.push(m.lineNumber)
	m.lineNumber = 0
}

// FIXME: inline this into back().
func (m *Model) restoreLineNumber() {
	val, err := lineNumberStack.pop()
	if err != nil {
		panic("FIXME: save errors to m.err")
	}

	m.lineNumber = val
}

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
	homeDir, err := os.UserHomeDir()

	// See [homeDirDepth]. For example, "/home/alice" here would
	// report a homeDirDepth of 2.
	homeDirDepth = len(strings.Split(homeDir, "/"))

	if err != nil {
		return Model{}, fmt.Errorf("cannot create dirselect widget: %w", err)
	}

	digits := key.WithKeys("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
	return Model{
		id:         nextID(),
		homeDir:    homeDir,
		currentDir: homeDir,
		keyMap: keyMap{
			up:           key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k/↑/ctrl+p", "previous line")),
			down:         key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j/↓/ctrl+n", "next line")),
			back:         key.NewBinding(key.WithKeys("h", "left", "ctrl+b"), key.WithHelp("h/←/ctrl+b", "go to parent directory")),
			explore:      key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("l/→/enter", "explore this directory")),
			jump:         key.NewBinding(digits, key.WithHelp("0-9", "jump to selection")),
			jumpToHome:   key.NewBinding(key.WithKeys("~"), key.WithHelp("~", "jump back to home directory")),
			toggleSelect: key.NewBinding(key.WithKeys(" "), key.WithHelp("spacebar", "toggle selection")),
			quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit")),
		},
		SelectedDirs: make([]string, 0),
	}, nil
}

// readDir returns a [tea.Cmd] that tells the UI to update with the
// list of directories corresponding to a users selection ('explore',
// 'back', etc.)
//
// Its sole purpose is to supply the path argument to the underlying
// closure, which is then returned as the actual command.
func (m Model) readDir(path string) tea.Cmd {
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

		return readDirMsg{id: m.id, entries: dirs}
	}
}

// dirAtPoint is sugar for returning the directory that the cursor is
// currently resting on. The directory is returned as an absolute
// path, per how [Model.currentDir] is always set.
//
// Note that we never edit the entries themselves, so it's OK for us
// to only have a getter method for this field.
func (m Model) dirAtPoint() string {
	return filepath.Join(m.currentDir, m.dirListing[m.lineNumber])
}

// back adjusts all the state necessary to effect a "cd .." operation.
func (m Model) back() (tea.Model, tea.Cmd) {
	m.currentDir = filepath.Dir(m.currentDir)
	m.restoreLineNumber()

	return m, m.readDir(m.currentDir)
}

func (m Model) explore() (tea.Model, tea.Cmd) {
	m.currentDir = m.dirAtPoint()
	m.saveLineNumber()
	return m, m.readDir(m.currentDir)
}

func (m Model) Init() tea.Cmd {
	return m.readDir(m.currentDir)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case readDirMsg:
		if msg.id != m.id {
			break
		}

		m.dirListing = msg.entries

		// FIXME: use const for magic number 10 here.
		m.viewHeight = min(10, len(m.dirListing))
		m.viewMin = 0
		m.viewMax = m.viewHeight - 1

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.back):
			if lineNumberStack.depth() == 0 {
				break
			}

			return m.back()

		case key.Matches(msg, m.keyMap.down):
			m.lineNumber++
			if m.lineNumber > len(m.dirListing)-1 {
				m.lineNumber = len(m.dirListing) - 1
			}

			if m.lineNumber > m.viewMax {
				m.viewMin++
				m.viewMax++
			}

		case key.Matches(msg, m.keyMap.explore):
			if m.lineNumber == 0 && lineNumberStack.depth() == 0 {
				break
			}

			if m.lineNumber == 0 {
				return m.back()
			}

			return m.explore()

		case key.Matches(msg, m.keyMap.jump):
			index, err := strconv.Atoi(msg.String())
			if err != nil {
				panic("FIXME: save error to m.err")
			}

			selection := m.SelectedDirs[index]

			// Note that the selection's parent directory
			// is the actual destination of the jump.
			m.currentDir = filepath.Dir(selection)

			// Upward navigation to parent directories
			// depends entirely on the stack, so we need a
			// dummy stack to reenable it.
			lineNumberStack = newStack()
			totalDepth := len(strings.Split(selection, "/"))

			for range totalDepth - homeDirDepth - 1 {
				lineNumberStack.push(0)
			}

			// Set the current line number to that of the
			// selection we've jumped to. This also helps
			// us avoid out-of-bound index panics.
			return m, m.readDir(m.currentDir)

		case key.Matches(msg, m.keyMap.jumpToHome):
			m.currentDir = m.homeDir
			m.lineNumber = 0
			lineNumberStack.reset()

			return m, m.readDir(m.currentDir)

		case key.Matches(msg, m.keyMap.toggleSelect):
			// Disable selection of the ".." entry. In
			// addition, no "[ ]" should appear next to
			// it.
			if m.lineNumber == 0 {
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
				m.SelectedDirs = append(m.SelectedDirs, dir)
			}

		case key.Matches(msg, m.keyMap.up):
			m.lineNumber--
			if m.lineNumber < 0 {
				m.lineNumber = 0
			}

			if m.lineNumber < m.viewMin {
				m.viewMin--
				m.viewMax--
			}

		case key.Matches(msg, m.keyMap.quit):
			logFile.Close()
			quitting = true
			return m, tea.Quit
		}
	}

	log.Printf("Uncaught message: %v", msg)
	return m, nil
}

var borderStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("63")).
	Width(50)

var entryStyle = lipgloss.NewStyle().
	MarginLeft(10)

var arrowStyle = entryStyle.Width(30).Align(lipgloss.Center)

var (
	upArrow   = arrowStyle.Render("↑")
	downArrow = arrowStyle.Render("↓")
)

func (m Model) View() string {
	// Don't render anything in thise case; see [quitting].
	if quitting {
		return ""
	}

	var view strings.Builder
	const (
		markChecked = "✓"
		markEmpty   = " "
	)

	// FIXME: this is for debugging only. It should be removed
	// when making a release.
	fmt.Fprintf(&view, "depth: %d\n\n", lineNumberStack.depth())

	// Display the "jump list."
	for i, s := range m.SelectedDirs {
		fmt.Fprintf(&view, "%d: %s\n", i, s)
	}

	if m.viewMin > 0 {
		view.WriteString(upArrow)
	} else {
		view.WriteString(arrowStyle.Render(""))
	}

	// Add a newline before listing entries, so that introducing
	// "↑" doesn't make the view look janky.
	view.WriteString("\n")

	for i, d := range m.dirListing {
		if i < m.viewMin || i > m.viewMax {
			continue
		}

		mark := markEmpty

		if slices.Contains(m.SelectedDirs, filepath.Join(m.currentDir, d)) {
			mark = markChecked
		}

		var entry string
		if i == m.lineNumber {
			entry = fmt.Sprintf("→ [%s] %s", mark, d)
		} else {
			entry = fmt.Sprintf("  [%s] %s", mark, d)
		}

		view.WriteString(entryStyle.Render(entry) + "\n")
	}

	if m.viewMax < len(m.dirListing)-1 {
		view.WriteString(downArrow)
	} else {
		view.WriteString(arrowStyle.Render(""))
	}

	return borderStyle.Render(view.String())
}
