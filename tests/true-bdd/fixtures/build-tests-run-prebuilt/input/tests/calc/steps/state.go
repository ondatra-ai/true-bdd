package steps

type State struct { EngineBin string; Help string }
type Registrar interface { Step(string, func(*State, []string) error) }
