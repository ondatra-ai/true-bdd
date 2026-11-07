package steps

import "fixture/stepsrun/tests/calc/helpers"

// State is what the calc suite's steps share within one scenario.
type State struct {
	// Calculator is the calculator program the installation Given built,
	// kept so the When can run it.
	Calculator *helpers.Calculator
	// Doubled is what the doubling When's run of the calculator printed,
	// kept so a Then can observe the integer the program reported. It is
	// nil until that When has run.
	Doubled *helpers.Doubled
}
