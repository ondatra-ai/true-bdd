package steps

import ("fmt"; "os/exec"; "strconv"; "strings")

//truebdd:concept when "halving"
func halving(state *State, args []string) error {
	tool := state.Go
	if tool == "" {
		tool = "go"
	}
	cmd := exec.Command(tool, "run", "./services/calc", "halve", args[0])
	cmd.Dir = "../.."
	cmd.Env = state.Env
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("halve %s: %w", args[0], err)
	}
	state.Result, err = strconv.Atoi(strings.TrimSpace(string(out)))
	return err
}
