package steps

// State carries what one scenario's steps share.
type State struct {
	// CalcPath is the path of the calculator program built by the
	// installation Given, kept for the When to run.
	CalcPath string

	// Doubled is the integer the kept calculator program's double
	// command printed on stdout, kept by the doubling When for the
	// result Then to observe.
	Doubled int

	// DoubledSet records that the doubling When ran the double command
	// and kept its integer stdout in Doubled.
	DoubledSet bool
}
