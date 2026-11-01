package steps

type State struct{ Output string }
type Registrar interface {
	Step(string, func(*State, []string) error)
}
