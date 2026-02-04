package dirselect

type Model struct {
	// The SelectedDirs field is the set of directories currently
	// selected by the user for use as the model's result
	// value. Directory selection is managed through
	// [keyMap.toggleSelect].
	//
	// The directories themselves are stored here as absolute
	// paths, so that they may be uniquely identified later on.
	SelectedDirs []string

	// The currentDir field is the path of the currently explored
	// directory. This should always be an absolute path. In
	// practice, this should always be the case since the
	// top-level always initializes it with the result of
	// [os.UserHomeDir].
	currentDir string
}

type readDirMsg struct {
	id       int
	entries  []string
	path     string
	startDir string
}
