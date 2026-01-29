package dirselect

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func New() (Model, error) {
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
		SelectedDirs: make(map[string]struct{}),
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
// currently resting on.
//
// Note that we never edit the entries themselves, so it's OK for us
// to only have a getter method for this field.
func (m Model) dirAtPoint() string {
	return m.dirListing[m.lineNumber]
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

	case tea.KeyMsg:
		switch {
		// The operations [keyMap.back] and [keyMap.explore]
		// involve a number of steps each:
		//
		// - Adjust [Model.depth], checking for an illegal
		// depth value where applicable.
		//
		// - Set [Model.currentDir] to either the parent or
		// child directory.
		//
		// - Reset [Model.lineNumber] to 0.
		//
		// - Return the model, along with a readDir command
		// for the updated [Model.currentDir].
		case key.Matches(msg, m.keyMap.back):
			m.depth--
			if m.depth < 0 {
				m.depth = 0
				return m, m.readDir(m.currentDir)
			}

			m.currentDir = filepath.Dir(m.dirAtPoint())
			m.lineNumber = 0
			return m, m.readDir(m.currentDir)

		case key.Matches(msg, m.keyMap.down):
			m.lineNumber++
			if m.lineNumber > len(m.dirListing)-1 {
				m.lineNumber = len(m.dirListing) - 1
			}

		case key.Matches(msg, m.keyMap.explore):
			// Don't do anything if we're on the ".."
			// entry of the top-level directory.
			if m.lineNumber == 0 && m.depth == 0 {
				break
			}

			if m.lineNumber == 0 {
				// We're going up, so decrease the
				// depth.
				m.depth--
			} else {
				// In the normal case, we're going
				// down, so increase the depth.
				m.depth++
			}

			// Note that, even in the case of "..",
			// [filepath.Join] will Clean the directory,
			// so we're good.
			m.currentDir = filepath.Join(m.currentDir, m.dirAtPoint())
			m.lineNumber = 0
			return m, m.readDir(m.currentDir)

		case key.Matches(msg, m.keyMap.jump):
			// FIXME: not implemented.
			log.Println(msg)

		case key.Matches(msg, m.keyMap.jumpToHome):
			m.currentDir = m.homeDir
			m.lineNumber = 0
			m.depth = 0

			return m, m.readDir(m.currentDir)

		case key.Matches(msg, m.keyMap.toggleSelect):
			absDir, err := filepath.Abs(m.dirAtPoint())
			if err != nil {
				return m, tea.Quit
			}

			// Toggle the presence of the directory in the
			// map.
			if _, present := m.SelectedDirs[absDir]; present {
				delete(m.SelectedDirs, absDir)
			} else {
				m.SelectedDirs[absDir] = struct{}{}
			}

		case key.Matches(msg, m.keyMap.up):
			m.lineNumber--
			if m.lineNumber < 0 {
				m.lineNumber = 0
			}

		case key.Matches(msg, m.keyMap.quit):
			return m, tea.Quit
		}

	}

	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	for i, d := range m.dirListing {
		checkMark := " "
		absDir, err := filepath.Abs(d)
		if err != nil {
			panic("FIXME: set up assigning errors to m.err")
		}

		if _, present := m.SelectedDirs[absDir]; present {
			checkMark = "✓"
		}

		if i == m.lineNumber {
			fmt.Fprintf(&s, "> [%s] %s\n", checkMark, d)
		} else {
			fmt.Fprintf(&s, "  [%s] %s\n", checkMark, d)
		}
	}

	return s.String()
}
