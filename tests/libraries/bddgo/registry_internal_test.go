package bddgo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeDoc(t *testing.T, name, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)

	err := os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	return path
}

// Given/When/Then are flattened into one ordered list, and every step
// after the first in a block reads as And — so one definition serves
// `the file is written` whether it is the first Then or the third.
func TestLoadRegistryFlattensBlocksInOrder(t *testing.T) {
	t.Parallel()

	path := writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    merged_steps:
      given:
        - a tree
        - another tree
      when:
        - the CLI runs
      then:
        - it exits 0
        - but: nothing else changed
`)

	scenarios, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	if len(scenarios) != 1 {
		t.Fatalf("want 1 scenario, got %d", len(scenarios))
	}

	if scenarios[0].ID != firstScenario {
		t.Fatalf("want %s, got %s", firstScenario, scenarios[0].ID)
	}

	want := []Step{
		{Keyword: KeywordGiven, Text: stepATree},
		{Keyword: KeywordAnd, Text: "another tree"},
		{Keyword: KeywordWhen, Text: stepCLIRuns},
		{Keyword: KeywordThen, Text: "it exits 0"},
		{Keyword: KeywordBut, Text: "nothing else changed"},
	}

	got := scenarios[0].Steps
	if len(got) != len(want) {
		t.Fatalf("want %d steps, got %d: %v", len(want), len(got), got)
	}

	for index, step := range want {
		if got[index] != step {
			t.Errorf("step %d: got %+v, want %+v", index, got[index], step)
		}
	}
}

// Scenarios load in id order, not map order, so two runs of the same
// registry walk the same sequence.
func TestLoadRegistrySortsByID(t *testing.T) {
	t.Parallel()

	path := writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-003: {description: c, service: cli, user_stories: [], merged_steps: {when: [x]}}
  E2E-001: {description: a, service: cli, user_stories: [], merged_steps: {when: [x]}}
  E2E-002: {description: b, service: cli, user_stories: [], merged_steps: {when: [x]}}
`)

	scenarios, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	for index, want := range []string{firstScenario, "E2E-002", "E2E-003"} {
		if scenarios[index].ID != want {
			t.Errorf("position %d: got %s, want %s", index, scenarios[index].ID, want)
		}
	}
}

// An empty registry is a document that says nothing, not a project with
// nothing to prove — the same refusal the engine makes.
func TestLoadRegistryRejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := LoadRegistry(writeDoc(t, "scenarios.yaml", "scenarios: {}\n"))
	if !errors.Is(err, ErrNoScenarios) {
		t.Fatalf("want ErrNoScenarios, got %v", err)
	}
}

// A step mapping with a key other than and/but is malformed rather than
// coerced: a step the loader guesses at is a step no reader can predict.
func TestLoadRegistryRejectsUnknownStepModifier(t *testing.T) {
	t.Parallel()

	_, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    merged_steps:
      then:
        - however: nothing else changed
`))
	if !errors.Is(err, ErrMalformedStep) {
		t.Fatalf("want ErrMalformedStep, got %v", err)
	}
}

// A model-run prefix is peeled off the text and recorded as a mode. The
// prefix must not survive into Text: a `judge:` clause is read by a model
// as a sentence, and a step definition is bound by wording.
func TestLoadRegistryClassifiesModelRunSteps(t *testing.T) {
	t.Parallel()

	scenarios, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    merged_steps:
      when:
        - 'llm: close whatever dialog is covering the list'
      then:
        - the page shows "3 sessions"
        - but: 'judge:   the wording still names the workspace  '
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	want := []Step{
		{Keyword: KeywordWhen, Mode: ModeAct, Text: "close whatever dialog is covering the list"},
		{Keyword: KeywordThen, Mode: ModeDeterministic, Text: `the page shows "3 sessions"`},
		{Keyword: KeywordBut, Mode: ModeRule, Text: "the wording still names the workspace"},
	}

	got := scenarios[0].Steps
	if len(got) != len(want) {
		t.Fatalf("want %d steps, got %d: %v", len(want), len(got), got)
	}

	for index, step := range want {
		if got[index] != step {
			t.Errorf("step %d: want %+v, got %+v", index, step, got[index])
		}
	}
}

// `judge:` rules on what already happened, so a given/when block cannot
// host it. Refused rather than reinterpreted as an act — the two prefixes
// name two different engines.
func TestLoadRegistryRejectsRulePrefixOutsideThen(t *testing.T) {
	t.Parallel()

	_, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    merged_steps:
      when:
        - 'judge: the page looks right'
`))
	if !errors.Is(err, ErrPrefixWrongBlock) {
		t.Fatalf("want ErrPrefixWrongBlock, got %v", err)
	}
}

// `llm:` acts, and a then: block asserts. Same refusal from the other
// side, so neither prefix can be pasted into the wrong block and quietly
// change what runs it.
func TestLoadRegistryRejectsActPrefixInThen(t *testing.T) {
	t.Parallel()

	_, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    merged_steps:
      then:
        - 'llm: close the dialog'
`))
	if !errors.Is(err, ErrPrefixWrongBlock) {
		t.Fatalf("want ErrPrefixWrongBlock, got %v", err)
	}
}

// The clause IS the whole instruction the model receives. An empty one
// asks it to rule on nothing, and would pass or fail on its mood.
func TestLoadRegistryRejectsEmptyClause(t *testing.T) {
	t.Parallel()

	_, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    merged_steps:
      then:
        - 'judge:   '
`))
	if !errors.Is(err, ErrEmptyClause) {
		t.Fatalf("want ErrEmptyClause, got %v", err)
	}
}

// A timeout that is not a positive duration is refused rather than
// rounded up to the suite default: a typo'd budget silently becoming the
// default is the substitution this repository refuses everywhere else.
func TestLoadRegistryRejectsUnparsableTimeout(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"ten minutes", "0s", "-5m"} {
		_, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: one
    service: cli
    user_stories: []
    timeout: "`+value+`"
    merged_steps:
      then:
        - it works
`))
		if !errors.Is(err, ErrTimeoutInvalid) {
			t.Errorf("timeout %q: want ErrTimeoutInvalid, got %v", value, err)
		}
	}
}

// An absent timeout is zero, which the suite reads as "use your default"
// — the overwhelmingly common case, and the one a refusal must not catch.
func TestLoadRegistryAcceptsTimeout(t *testing.T) {
	t.Parallel()

	scenarios, err := LoadRegistry(writeDoc(t, "scenarios.yaml", `
scenarios:
  E2E-001:
    description: budgeted
    service: cli
    user_stories: []
    timeout: 10m
    merged_steps:
      then: [it works]
  E2E-002:
    description: default
    service: cli
    user_stories: []
    merged_steps:
      then: [it works]
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if scenarios[0].Timeout != 10*time.Minute {
		t.Errorf("E2E-001: want 10m, got %v", scenarios[0].Timeout)
	}

	if scenarios[1].Timeout != 0 {
		t.Errorf("E2E-002: want zero, got %v", scenarios[1].Timeout)
	}
}
