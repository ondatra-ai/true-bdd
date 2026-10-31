package steps

import "strconv"

//truebdd:concept when "doubling"
func concept49c5ece16061081405dd19dacacf9182109378be8a0f1d472e9711fff19b790b(state *State, args []string) error {
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	state.Value = n * 2
	return nil
}
