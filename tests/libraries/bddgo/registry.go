// Package bddgo runs the scenarios in the central registry
// (`documents.scenarios_yaml`, conventionally docs/scenarios.yaml) as Go
// tests.
//
// It is cucumber's model with the feature files taken out. In godog a
// scenario lives in a .feature file and a step definition binds a regexp
// to Go code; here the scenario already exists — `us apply` merged it
// into the registry, and `build tests` walks it — so the only thing
// missing was the binding. A suite registers its step definitions, and
// every registry scenario the architectural spec says it owns becomes a
// subtest.
//
// The consequence worth stating: a scenario with a step no definition
// matches FAILS. It is not skipped and it is not silently absent from
// the run. That failure is the whole contract `build tests` enforces —
// "this behaviour is written down and nothing executes it" — and a skip
// would let a green suite hide it.
package bddgo

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNoScenarios signals a registry with no scenarios at all. Mirrors
// the engine's own refusal: an empty registry is a document that says
// nothing, not a project with nothing to prove.
var ErrNoScenarios = errors.New("scenario registry declares no scenarios")

// ErrMalformedStep signals a step that is neither a plain string nor a
// single-key `{and|but: "..."}` mapping — the two shapes `us apply`
// writes.
var ErrMalformedStep = errors.New("step must be a string or a single and/but mapping")

// Keywords a step can carry. Given/When/Then come from the block the
// step sits in; And/But are the modifiers a step may spell out for
// itself, and they inherit the meaning of the step above.
const (
	KeywordGiven = "Given"
	KeywordWhen  = "When"
	KeywordThen  = "Then"
	KeywordAnd   = "And"
	KeywordBut   = "But"
)

// Step is one Given/When/Then line of a scenario.
//
// Text is what a step definition's pattern is matched against, and the
// modifier is deliberately NOT part of it: `and the file is written` and
// `the file is written` are the same assertion said twice, so one
// definition must serve both.
type Step struct {
	Keyword string
	Text    string
}

// String renders the step the way a reader wrote it, which is also the
// form a failure quotes back.
func (s Step) String() string {
	return s.Keyword + " " + s.Text
}

// Scenario is one entry under `scenarios:` in the registry.
type Scenario struct {
	ID          string
	Description string
	Service     string
	Requirement string
	Feature     string
	Steps       []Step
}

// rawStep decodes the two shapes a step may take.
type rawStep struct {
	Keyword string
	Text    string
}

// UnmarshalYAML accepts a scalar step or a one-key `and:`/`but:`
// mapping. Anything else is malformed rather than coerced: a step the
// loader guesses at is a step no reader can predict the behaviour of.
func (r *rawStep) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		r.Text = node.Value

		return nil
	}

	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return fmt.Errorf("%w (line %d)", ErrMalformedStep, node.Line)
	}

	key := strings.ToLower(node.Content[0].Value)
	if key != "and" && key != "but" {
		return fmt.Errorf("%w: got %q (line %d)", ErrMalformedStep, key, node.Line)
	}

	r.Keyword = strings.ToUpper(key[:1]) + key[1:]
	r.Text = node.Content[1].Value

	return nil
}

// rawMergedSteps mirrors the `merged_steps:` block of a registry entry.
type rawMergedSteps struct {
	Given []rawStep `yaml:"given,omitempty"`
	When  []rawStep `yaml:"when,omitempty"`
	Then  []rawStep `yaml:"then,omitempty"`
}

// rawScenario mirrors one value under `scenarios:`.
type rawScenario struct {
	Description string         `yaml:"description"`
	Service     string         `yaml:"service"`
	Requirement string         `yaml:"requirement,omitempty"`
	Feature     string         `yaml:"feature,omitempty"`
	MergedSteps rawMergedSteps `yaml:"merged_steps"`
}

// rawRegistry mirrors the top-level shape of the registry document.
type rawRegistry struct {
	Scenarios map[string]rawScenario `yaml:"scenarios"`
}

// LoadRegistry reads the scenario registry and returns every scenario in
// it, sorted by id so a run's order is a property of the document rather
// than of Go's map iteration.
func LoadRegistry(path string) ([]Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenario registry %s: %w", path, err)
	}

	var raw rawRegistry

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("parse scenario registry %s: %w", path, err)
	}

	if len(raw.Scenarios) == 0 {
		return nil, fmt.Errorf("%s: %w", path, ErrNoScenarios)
	}

	scenarios := make([]Scenario, 0, len(raw.Scenarios))

	for id, entry := range raw.Scenarios {
		scenarios = append(scenarios, Scenario{
			ID:          id,
			Description: entry.Description,
			Service:     entry.Service,
			Requirement: entry.Requirement,
			Feature:     entry.Feature,
			Steps:       flattenSteps(entry.MergedSteps),
		})
	}

	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ID < scenarios[j].ID })

	return scenarios, nil
}

// flattenSteps renders the three blocks into one ordered list. A step
// that named its own `and`/`but` keeps it; the first step of each block
// takes the block's keyword and the rest default to `And`, which is how
// the same lines read on a feature file.
func flattenSteps(steps rawMergedSteps) []Step {
	out := make([]Step, 0, len(steps.Given)+len(steps.When)+len(steps.Then))

	blocks := []struct {
		keyword string
		steps   []rawStep
	}{
		{KeywordGiven, steps.Given},
		{KeywordWhen, steps.When},
		{KeywordThen, steps.Then},
	}

	for _, block := range blocks {
		for index, step := range block.steps {
			out = append(out, Step{
				Keyword: stepKeyword(step, block.keyword, index),
				Text:    strings.TrimSpace(step.Text),
			})
		}
	}

	return out
}

func stepKeyword(step rawStep, blockKeyword string, index int) string {
	if step.Keyword != "" {
		return step.Keyword
	}

	if index == 0 {
		return blockKeyword
	}

	return KeywordAnd
}
