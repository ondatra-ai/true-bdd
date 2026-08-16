package bddgo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
// after the first in a block reads as And — which is what makes one
// definition serve `the file is written` whether it is the first Then or
// the third.
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
		{Keyword: KeywordGiven, Text: "a tree"},
		{Keyword: KeywordAnd, Text: "another tree"},
		{Keyword: KeywordWhen, Text: "the CLI runs"},
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
