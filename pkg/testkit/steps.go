package testkit

import "github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"

// StepRegistrar accepts patterns and their definition-owned bodies.
type StepRegistrar[S any] interface {
	// Step registers a pattern and its definition-owned body.
	Step(pattern string, run func(*S, []string) error)
}

// Judgeable is the state capability for judging a Scenario's collected clauses.
type Judgeable = bddgo.Judgeable

// Scenario is one Registry entry, including its ordered steps and product context.
type Scenario = bddgo.Scenario

// ModeDeterministic marks a step executed by a registered definition.
const ModeDeterministic = bddgo.ModeDeterministic

// LoadRegistry reads the host's declared Scenarios without executing their steps.
func LoadRegistry(path string) ([]Scenario, error) { return bddgo.LoadRegistry(path) }
