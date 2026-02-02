package dirselect

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

var ErrEmptyStack = errors.New("empty lineNumberStack")

type stack struct {
	push  func(int)
	pop   func() (int, error)
	depth func() int
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
	}
}

var stack2 = newStack()

type lineNumberStack struct {
	values []int
}

func (s *lineNumberStack) reset() {
	s.values = make([]int, 0)
}

func (m *Model) saveLineNumber() {
	stack2.push(m.lineNumber)
	m.lineNumber = 0
}

func (m *Model) restoreLineNumber() {
	val, err := stack2.pop()
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
		SelectedDirs:    make([]string, 0),
		lineNumberStack: lineNumberStack{values: make([]int, 0)},
		logFile:         logFile,
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
			if m.lineNumberStack.depth() == 0 {
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
			if m.lineNumber == 0 && m.lineNumberStack.depth() == 0 {
				break
			}

			if m.lineNumber == 0 {
				return m.back()
			}

			return m.explore()

		case key.Matches(msg, m.keyMap.jump):
			// FIXME: not implemented.
			log.Println(msg)

		case key.Matches(msg, m.keyMap.jumpToHome):
			m.currentDir = m.homeDir
			m.lineNumber = 0
			m.lineNumberStack.reset()

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
			m.logFile.Close()
			return m, tea.Quit
		}
	}

	log.Printf("Uncaught message: %v", msg)
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder
	const (
		markChecked = "✓"
		markEmpty   = " "
	)

	s.WriteString("\n")
	fmt.Fprintf(&s, "depth: %d\n\n", m.lineNumberStack.depth())

	for i, d := range m.dirListing {
		if i < m.viewMin || i > m.viewMax {
			continue
		}

		mark := markEmpty

		if slices.Contains(m.SelectedDirs, filepath.Join(m.currentDir, d)) {
			mark = markChecked
		}

		if i == m.lineNumber {
			fmt.Fprintf(&s, "> [%s] %s\n", mark, d)
		} else {
			fmt.Fprintf(&s, "  [%s] %s\n", mark, d)
		}
	}

	return s.String()
}
