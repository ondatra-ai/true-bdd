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
	// Result is the integer the calculator's double command printed on
	// stdout, kept by the doubling When for the result Then to observe.
	Result int
	// Doubled records that the doubling When ran the double command and
	// kept its stdout, so the result Then can tell a printed 0 apart from
	// no run at all.
	Doubled bool
}
