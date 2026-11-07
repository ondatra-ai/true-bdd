package steps

import (
	"fmt"

	"fixture/conceptbodiescases/tests/calc/helpers"
)

//truebdd:concept when "operation"
func concept40523ceb26a25e37e78222ac562623f48c38b52711218d94fe5cadea337eebba(state *State, args []string) error {
	exact, operation, number := args[0], args[1], args[2]

	command, err := helpers.Command(operation)
	if err != nil {
		return err
	}

	var flags []string
	if exact != "" {
		if command != helpers.HalveCommand {
			return fmt.Errorf("the declared %s command takes no %s flag", command, helpers.ExactFlag)
		}
		flags = append(flags, helpers.ExactFlag)
	}

	output, err := helpers.Run(command, append(flags, number)...)
	if err != nil {
		return err
	}

	state.Output = output
	return nil
}
