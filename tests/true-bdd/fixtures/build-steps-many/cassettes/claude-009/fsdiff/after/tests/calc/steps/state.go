package steps

import "fixture/stepsrun/tests/calc/helpers"

// State is what the calc suite's steps share within one scenario.
type State struct {
	// Calculator is the calculator program the installation Given built,
	// kept so the When can run it.
	Calculator *helpers.Calculator
}
