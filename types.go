package dirselect

import (
	"github.com/charmbracelet/bubbles/key"
)

type Model struct {
	// The id field is the reference-count id of this model.
	id int

	// The lineNumber field is the zero-indexed line number of the
	// current selection.
	lineNumber int

	// The selectedDirs field is the list of directories currently
	// selected by the user for use as the model's result value.
	selectedDirs []string

	// The dirListing field is the list of directories inside the
	// currently explored directory. It always has at least one
	// entry, '..', allowing the user to navigate to the parent
	// directory. This is partly to avoid panics involving empty
	// slices in the case of otherwise empty directories.
	dirListing []string

	// The currentDir field is the absolute path of the currently
	// explored directory.
	currentDir string

	// The cursor field is the shape of the cursor denoting the
	// current line number (e.g., '>')
	cursor string

	// The keyMap field is the set of keybindings in for
	// navigating the model UI (e.g., '↑' goes to the previous
	// line, etc.)
	keyMap keyMap

	// The depth field tracks how deep we've gone beneath the
	// starting directory. It's illegal to move above a depth of
	// 0.
	depth int
}

// KeyMap defines key bindings for each user action.
type keyMap struct {
	down         key.Binding
	up           key.Binding
	back         key.Binding
	explore      key.Binding
	toggleSelect key.Binding
	jump         key.Binding
	quit         key.Binding
}

type readDirMsg struct {
	id      int
	entries []string
}
