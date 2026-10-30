package steps

import ("fmt"; "strconv")

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
