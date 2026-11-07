package steps

import (
	"fmt"

	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("expected 3 captures (exactly, operation, number), got %d: %q", len(args), args)
	}
	exactly, operation, number := args[0], args[1], args[2]

	command, err := helpers.OperationCommand(operation)
	if err != nil {
		return err
	}

	var flags []string
	if exactly != "" {
		if command != helpers.CommandHalve {
			return fmt.Errorf("the calculator's %s command declares no %s flag, so %q cannot be run exactly", command, helpers.ExactFlag, operation)
		}
		flags = append(flags, helpers.ExactFlag)
	}

	output, err := helpers.RunCalc(command, append(flags, number)...)
	if err != nil {
		return err
	}

	state.Output = output

	return nil
}
