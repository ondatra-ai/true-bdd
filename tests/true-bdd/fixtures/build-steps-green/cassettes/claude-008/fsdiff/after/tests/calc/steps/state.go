package steps

type State struct {
	Value int
}
type Registrar interface {
	Step(string, func(*State, []string) error)
}
