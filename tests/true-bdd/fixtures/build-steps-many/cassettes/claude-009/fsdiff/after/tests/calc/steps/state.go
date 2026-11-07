package steps

// State is what the calc suite's steps share within one scenario.
type State struct {
	// CalculatorPath is the calculator program that the installation Given
	// built under the host's tmp directory, kept so the When can run it.
	CalculatorPath string
}
