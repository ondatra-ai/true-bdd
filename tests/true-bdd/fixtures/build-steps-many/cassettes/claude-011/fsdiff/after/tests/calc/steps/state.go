package steps

// State is what one scenario's calc steps share.
type State struct {
	// HostRoot is the host project's root — the directory holding the
	// go.mod of the module under test — located by the installation Given.
	// It is the directory the calculator program is run from.
	HostRoot string
	// Program is the path of the calculator program the installation Given
	// built under the host's tmp directory.
	Program string
}
