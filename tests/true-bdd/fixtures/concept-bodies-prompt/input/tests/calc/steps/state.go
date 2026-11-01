package steps

type State struct {
	Result int
	Go     string
	Env    []string
}

type Registrar interface {
	Step(string, func(*State, []string) error)
}
