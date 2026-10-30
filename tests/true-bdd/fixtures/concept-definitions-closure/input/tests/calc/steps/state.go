package steps

type State struct{ Result int }

type Registrar interface {
	Step(string, func(*State, []string) error)
}
