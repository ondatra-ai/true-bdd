package steps

// State is the calc suite's shared state.
type State struct {
	// CalcPath is the path of the calculator program built by the
	// installation Given, kept for the When that runs it.
	CalcPath string

	// Doubled is the integer the kept calculator program's double command
	// printed on stdout, kept by the doubling When so a Then can observe
	// and compare it. The suite never computes this value itself.
	Doubled int

	// DoubledSet reports whether the doubling When ran the double command
	// and recorded its stdout in Doubled, so an observing Then can tell a
	// recorded zero from a When that never ran.
	DoubledSet bool
}
