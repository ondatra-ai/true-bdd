package steps

import "fixture/conceptsteps/tests/calc/helpers"

//truebdd:concept when "halving"
func concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb(state *State, args []string) error {
	// args[0] is the step's negative whole number, minus sign and all,
	// passed verbatim as the halve command's VALUE argument. Running the
	// command is the whole of this when: the calculator halves and
	// rounds, and its integer stdout is kept as the result for the then
	// step to observe.
	result, err := helpers.Halve(args[0])
	if err != nil {
		return err
	}
	state.Result = result
	return nil
}
