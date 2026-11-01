package steps

type State struct {
	// Program is the filesystem path of the calculator program built by the
	// installation Given, kept for the When that runs it.
	Program string

	// HostRoot is the host's root directory (the one holding go.mod and
	// services/calc), the directory the declared interface says the command is
	// run from.
	HostRoot string
}
