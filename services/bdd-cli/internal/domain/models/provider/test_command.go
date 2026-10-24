package provider

// TestCommand grants crush the declared replay, optionally with one selector.
type TestCommand struct {
	Command  string
	Selector string
	Dir      string
}
