package steps

//truebdd:concept when "halving"
func concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb(state *State, args []string) error {
	// args[0] is the step's negative whole number, as written, and is passed
	// straight through as the halve command's value argument. halveNumber runs
	// the declared command as a process and keeps its integer stdout in
	// state.Result for the Then step to observe.
	return halveNumber(state, args)
}
