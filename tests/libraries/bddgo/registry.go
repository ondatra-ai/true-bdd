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
//
// Not every step can be settled by a regexp, though, and the ones that
// cannot say so in their own text: see model_steps.go for the `llm:` and
// `judge:` prefixes, which hand a step to a model instead of to a step
// definition. They exist so that the alternative — writing the
// unmatchable half of a specification somewhere outside the registry —
// stops being necessary.
package bddgo

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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

// ErrTimeoutInvalid signals a scenario `timeout:` that is not a positive
// Go duration.
var ErrTimeoutInvalid = errors.New("scenario timeout must be a positive Go duration")

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
// Text is what a step definition's pattern is matched against, and
// neither the modifier nor a model-run prefix is part of it: `and the
// file is written` and `the file is written` are the same assertion said
// twice, so one definition must serve both, and `llm:`/`judge:` say who
// runs the step rather than what it says.
type Step struct {
	Keyword string
	Mode    StepMode
	Text    string
}

// String renders the step the way a reader wrote it, which is also the
// form a failure quotes back.
func (s Step) String() string {
	switch s.Mode {
	case ModeAct:
		return s.Keyword + " " + PrefixAct + " " + s.Text
	case ModeRule:
		return s.Keyword + " " + PrefixRule + " " + s.Text
	case ModeDeterministic:
		return s.Keyword + " " + s.Text
	default:
		return s.Keyword + " " + s.Text
	}
}

// Scenario is one entry under `scenarios:` in the registry.
type Scenario struct {
	ID          string
	Description string
	Service     string
	// Path is the generated test file this scenario's Go test is written
	// into, repo-relative. Several scenarios may share one.
	//
	// bddgo does not validate it — by the time a test runs, the file it
	// was generated into is the one calling. Only the coverage guard
	// reads it, to catch a generated file someone moved by hand.
	Path        string
	Requirement string
	Feature     string
	// Timeout is the scenario's own budget for the work its When step
	// kicks off. Zero means the suite's default.
	//
	// A harness budget rather than behaviour, and the one key here that
	// is: how long a run may take is a fact about the machine it runs on,
	// so it cannot be asserted and does not belong in a step.
	Timeout time.Duration
	Steps   []Step
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
	Path        string         `yaml:"path,omitempty"`
	Requirement string         `yaml:"requirement,omitempty"`
	Feature     string         `yaml:"feature,omitempty"`
	Timeout     string         `yaml:"timeout,omitempty"`
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

	// Sorted before the loop, not after: a document with two bad
	// scenarios must name the same one every run, and Go's map iteration
	// would otherwise pick whichever it felt like.
	ids := make([]string, 0, len(raw.Scenarios))
	for id := range raw.Scenarios {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	scenarios := make([]Scenario, 0, len(ids))

	for _, id := range ids {
		entry := raw.Scenarios[id]

		scenario, err := buildScenario(id, entry)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

// buildScenario turns one decoded entry into a Scenario, refusing what
// the raw decode could not see: a timeout that is not a duration, and a
// model-run prefix in a block that cannot run it.
func buildScenario(scenarioID string, entry rawScenario) (Scenario, error) {
	timeout, err := parseScenarioTimeout(entry.Timeout)
	if err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", scenarioID, err)
	}

	steps, err := flattenSteps(entry.MergedSteps)
	if err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", scenarioID, err)
	}

	return Scenario{
		ID:          scenarioID,
		Description: entry.Description,
		Service:     entry.Service,
		Path:        entry.Path,
		Requirement: entry.Requirement,
		Feature:     entry.Feature,
		Timeout:     timeout,
		Steps:       steps,
	}, nil
}

// parseScenarioTimeout reads the optional `timeout:` key. Absent means
// zero, which the suite reads as "use your default"; a value that is not
// a positive duration is refused rather than rounded up to the default,
// because a typo'd budget silently becoming the default is exactly the
// substitution this repository refuses everywhere else.
func parseScenarioTimeout(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}

	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", ErrTimeoutInvalid, trimmed, err)
	}

	if timeout <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrTimeoutInvalid, trimmed)
	}

	return timeout, nil
}

// flattenSteps renders the three blocks into one ordered list. A step
// that named its own `and`/`but` keeps it; the first step of each block
// takes the block's keyword and the rest default to `And`, which is how
// the same lines read on a feature file.
//
// Whether a step is model-run is settled here too, against the BLOCK's
// keyword rather than the step's own: `- but: 'judge: …'` under then: is
// a rule, and the `But` it spelled is the reader's punctuation.
func flattenSteps(steps rawMergedSteps) ([]Step, error) {
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
			text := strings.TrimSpace(step.Text)

			mode, text, err := Classify(text, block.keyword)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", step.Text, err)
			}

			if mode != ModeDeterministic && text == "" {
				return nil, fmt.Errorf("%q: %w", step.Text, ErrEmptyClause)
			}

			out = append(out, Step{
				Keyword: stepKeyword(step, block.keyword, index),
				Mode:    mode,
				Text:    text,
			})
		}
	}

	return out, nil
}

// stepKeyword resolves one step's keyword. The BLOCK's keyword wins at
// index 0 even when the step spelled a modifier for itself, and the same
// rule lives in the engine's statementKeyword.
//
// It has to win, not merely read better. A generated file states each
// step by calling the method its keyword names, and Run.modify gives
// And/But the block OPENED above them — so a block emitted as `s.And(…)`
// first would inherit the previous block, and a `judge:` clause the
// loader classified against then: would be refused at run time as
// illegal in when:. `And` as the opening line of a block is not Gherkin
// anyway.
func stepKeyword(step rawStep, blockKeyword string, index int) string {
	if index == 0 {
		return blockKeyword
	}

	if step.Keyword != "" {
		return step.Keyword
	}

	return KeywordAnd
}
