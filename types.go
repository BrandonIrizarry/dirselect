package dirselect

type Model struct {
	// SelectedDirs is the set of directories currently selected
	// by the user for use as the model's result value. Directory
	// selection is managed through the toggleSelect field of the
	// [keyMap].
	//
	// The directories themselves are stored here as absolute
	// paths, so that they may be uniquely identified later on.
	SelectedDirs []string
}

type readDirMsg struct {
	entries  []string
	path     string
	startDir string
}
