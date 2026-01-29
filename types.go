package dirselect

import (
	"github.com/charmbracelet/bubbles/key"
)

type Model struct {
	// The SelectedDirs field is the set of directories currently
	// selected by the user for use as the model's result
	// value. Directory selection is managed through
	// [keyMap.toggleSelect].
	//
	// The directories themselves are stored here as absolute
	// paths, so that they may be uniquely identified later on.
	SelectedDirs map[string]struct{}

	// The id field is the reference-count id of this model.
	id int

	// The lineNumber field is the zero-indexed line number of the
	// current selection.
	lineNumber int

	// The dirListing field is the list of directories inside the
	// currently explored directory. It always has at least one
	// entry, '..', allowing the user to navigate to the parent
	// directory. This is partly to avoid panics involving empty
	// slices in the case of otherwise empty directories.
	dirListing []string

	// The homeDir field is the user's home directory, stored for
	// allowing jumps back to it.
	homeDir string

	// The currentDir field is the path of the currently explored
	// directory. This should always be an absolute path. In
	// practice, this should always be the case since the
	// top-level always initializes it with the result of
	// [os.UserHomeDir].
	currentDir string

	// The keyMap field is the set of keybindings in for
	// navigating the model UI (e.g., '↑' goes to the previous
	// line, etc.)
	keyMap keyMap

	// The depth field tracks how deep we've gone beneath the
	// directory with which the widget was first initialized
	// (i.e. the first value of [Model.currentDir]). It's illegal
	// to move above a depth of 0.
	//
	// This field effectively performs the same function as the
	// stack used in the original filepicker Bubble Tea component,
	// arguably in a simpler manner.
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
	jumpToHome   key.Binding
	quit         key.Binding
}

type readDirMsg struct {
	id      int
	entries []string
}
