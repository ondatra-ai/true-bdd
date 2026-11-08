package steps

// State is what the calc suite's steps share within one scenario.
type State struct {
	// CalculatorPath is the calculator program that the installation Given
	// built under the host's tmp directory, kept so the When can run it.
	CalculatorPath string

	// Doubled is the integer the calculator's double command printed on
	// stdout when the doubling When ran it: the program's own answer, kept
	// unchanged for the Then that observes the result to compare against the
	// expected value.
	Doubled int

	// DoubledStdout is that same stdout as the program printed it, kept so a
	// Then that disagrees can report the program's own text, and so a Then
	// can tell "the program printed 0" from "the When never ran".
	DoubledStdout string
}
