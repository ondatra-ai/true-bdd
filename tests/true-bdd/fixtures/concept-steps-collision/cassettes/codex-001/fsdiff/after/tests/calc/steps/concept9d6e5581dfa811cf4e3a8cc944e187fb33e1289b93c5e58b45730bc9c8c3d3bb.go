package steps

//truebdd:concept when "halving"
func concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb(state *State, args []string) error {
	// args[0] is the negative whole number, minus sign included, handed verbatim
	// to the declared halve command's value argument. halveNumber runs that
	// command as a process and keeps its integer stdout in state.Result.
	return halveNumber(state, args)
}
