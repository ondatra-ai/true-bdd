package steps

import ("fmt"; "os/exec"; "strconv"; "strings")

//truebdd:concept when "halving"
func concept9d6e5581dfa811cf4e3a8cc944e187fb33e1289b93c5e58b45730bc9c8c3d3bb(state *State, args []string) error {
	cmd := exec.Command("go", "run", "./services/calc", "halve", args[0])
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("halve %s: %w", args[0], err)
	}
	state.Result, err = strconv.Atoi(strings.TrimSpace(string(out)))
	return err
}
