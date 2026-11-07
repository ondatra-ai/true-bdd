package steps

import "fixture/conceptbodiescontract/tests/calc/helpers"

//truebdd:concept given "toolchain"
func concept47f741283eda70d7a0d1d2aaa90ad59f9b70d6cc753b4652aced0fd106d24dd9(state *State, args []string) error {
	toolchain, err := helpers.FixtureToolchain(args[0])
	if err != nil {
		return err
	}
	env, err := helpers.ToolchainEnv()
	if err != nil {
		return err
	}
	state.Go = toolchain
	state.Env = env
	return nil
}

