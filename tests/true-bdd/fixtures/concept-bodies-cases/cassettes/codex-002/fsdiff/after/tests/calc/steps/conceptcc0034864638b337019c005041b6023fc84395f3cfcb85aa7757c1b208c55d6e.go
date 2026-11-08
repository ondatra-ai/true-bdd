package steps

import (
	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept when "operation"
func conceptcc0034864638b337019c005041b6023fc84395f3cfcb85aa7757c1b208c55d6e(state *State, args []string) error {
	operation, number := args[0], args[1]

	// The step's operation phrase names which declared command to run and
	// whether the declared halve command gets its --exact flag; the phrase
	// "exactly doubles" can no longer reach here, so no body-side rejection
	// of an unmeant step form remains.
	command, flags, err := helpers.Operation(operation)
	if err != nil {
		return err
	}

	// Run the named command as a process with the step's whole number as its
	// VALUE argument and keep its stdout as the output. The calculator alone
	// halves or doubles; the body neither computes nor interprets the result.
	output, err := helpers.Run(command, append(flags, number)...)
	if err != nil {
		return err
	}

	state.Output = output
	return nil
}
