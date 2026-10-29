package steps

import ("strconv")

func Register(suite Registrar) {
 suite.Step(`^the number is (\d+)$`, number)
 suite.Step(`^the number is doubled$`, double)
}
func number(state *State, args []string) error { value, err := strconv.Atoi(args[0]); state.Value = value; return err }
