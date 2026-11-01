package steps

import ("fmt"; "os/exec"; "strconv"; "strings")

func halveNumber(state *State, args []string) error {
	cmd := exec.Command("go", "run", "./services/calc", "halve", args[0])
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("halve %s: %w", args[0], err)
	}
	state.Result, err = strconv.Atoi(strings.TrimSpace(string(out)))
	return err
}

func result(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	if state.Result != want {
		return fmt.Errorf("expected %d, got %d", want, state.Result)
	}
	return nil
}
