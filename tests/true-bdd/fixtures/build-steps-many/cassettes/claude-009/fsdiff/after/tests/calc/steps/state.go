package steps

// State is the calc suite's shared state.
type State struct {
	// CalcPath is the path of the calculator program built by the
	// installation Given, kept for the When that runs it.
	CalcPath string
}
