package steps

import "github.com/ondatra-ai/true-bdd/pkg/testkit"

type State struct{ *testkit.ScenarioRun }
type Registrar interface {
	Step(string, func(*State, []string) error)
}

func Register(Registrar) {}
