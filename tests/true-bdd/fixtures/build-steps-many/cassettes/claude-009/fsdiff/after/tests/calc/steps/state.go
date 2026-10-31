package steps

// State carries what one scenario's steps share.
type State struct {
	// CalcPath is the path of the calculator program built by the
	// installation Given, kept for the When to run.
	CalcPath string
}
