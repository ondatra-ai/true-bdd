package steps

import "github.com/ondatra-ai/true-bdd/pkg/testkit"

type State struct {
 *testkit.ScenarioRun
	Program string
	Value   int
}
type Registrar interface {
	Step(string, func(*State, []string) error)
}
