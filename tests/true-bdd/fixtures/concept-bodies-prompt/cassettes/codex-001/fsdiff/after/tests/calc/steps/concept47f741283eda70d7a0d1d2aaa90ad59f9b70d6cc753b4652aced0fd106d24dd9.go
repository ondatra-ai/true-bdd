package steps

import "fixture/conceptbodiesprompt/tests/calc/helpers"

//truebdd:concept given "toolchain"
func concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9(state *State, args []string) error {
	standIn, err := helpers.ToolchainStandIn(args[0])
	if err != nil {
		return err
	}
	realGo, err := helpers.RealGo()
	if err != nil {
		return err
	}
	state.Go = standIn
	state.Env = helpers.EnvWithRealGo(realGo)
	return nil
}
