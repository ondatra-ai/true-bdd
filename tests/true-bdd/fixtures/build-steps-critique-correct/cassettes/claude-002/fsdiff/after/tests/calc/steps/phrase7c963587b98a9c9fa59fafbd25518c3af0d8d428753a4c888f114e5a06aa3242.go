package steps

import "strconv"

func phrase7c963587b98a9c9fa59fafbd25518c3af0d8d428753a4c888f114e5a06aa3242(state *State, args []string) error {
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	state.Value = value
	return nil
}
