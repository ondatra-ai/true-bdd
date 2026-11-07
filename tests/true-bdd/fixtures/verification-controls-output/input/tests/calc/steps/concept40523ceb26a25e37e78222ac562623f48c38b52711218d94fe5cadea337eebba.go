package steps

import "fixture/conceptbodiescases/tests/calc/helpers"

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	// What the step named, read off its captures: the declared command for
	// "halves" or "doubles", whether --exact goes with it because the step
	// says the Owner halves exactly, and the whole number to pass as VALUE.
	command, exact, number, err := helpers.OperationFromStep(args)
	if err != nil {
		return err
	}

	// The calculator halves or doubles; this step only runs the declared
	// command as a process and keeps what it printed.
	output, err := helpers.RunCalc(command, exact, number)
	if err != nil {
		return err
	}

	state.Output = output

	return nil
}

