package steps

type State struct {
	Program string
	Value   int
}
type Registrar interface {
	Step(string, func(*State, []string) error)
}
