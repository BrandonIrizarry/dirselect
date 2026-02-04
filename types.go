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
	// entries comprises the listing of the directories individual
	// child directories. All directory listings contain at least
	// "..", which naturally points to the parent directory. In
	// the case of the top level directory (which in this
	// implmentation is chosen to be the user's home directory),
	// ".." is listed but not actionable.
	entries []string

	// currentDir is the directory whose child directories are
	// being displayed. This information gets used in various
	// operations and so the message needs to return it.
	currentDir string

	// startDir is used to find the appropriate position to scroll
	// down to when first listing a directory. For example, when
	// going up a directory, we want the cursor to land on the
	// current directory's entry in the parent directory's
	// listing. Luckily, we can determine what the entry is by
	// applying [filepath.Base] to the current directory, doing
	// away with any state buffers (stacks, etc.):
	//
	// Inside /home/user/cherry:
	//
	// marmalade
	// jam
	// syrup
	// etc.
	//
	// Back up to /home/user:
	//
	// grape
	// guava
	// cherry   <---- filepath.Base("/home/user/cherry")
	startDir string
}
