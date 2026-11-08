package steps

type State struct{}
type Registrar interface { Step(string, func(*State, []string) error) }
func Register(Registrar) {}
