package registry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeRegistry lays down a registry document whose one scenario has the
// given `given:` block, and returns its path.
func writeRegistry(t *testing.T, givenBlock string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "scenarios.yaml")

	doc := `metadata: {title: t, version: "1", description: d}
scenarios:
  INT-900:
    description: d
    service: s
    path: tests/x/x_test.go
    user_stories: []
    merged_steps:
      given:
` + givenBlock

	err := os.WriteFile(path, []byte(doc), 0o600)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

// The two modifiers, and their keywords survive the decode separately
// from their text — which is what lets the generator emit s.And(…)
// rather than a step whose first word happens to be "and".
func TestLoadKeepsModifierKeywords(t *testing.T) {
	t.Parallel()

	scenarios, err := NewRegistryLoader().Load(writeRegistry(t,
		"        - a thing\n        - and: another\n        - but: not that\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := []Statement{
		{Keyword: KeywordGiven, Text: "a thing"},
		{Keyword: KeywordAnd, Text: "another"},
		{Keyword: KeywordBut, Text: "not that"},
	}

	got := scenarios[0].Statements
	if len(got) != len(want) {
		t.Fatalf("got %d statements, want %d: %+v", len(got), len(want), got)
	}

	for index, statement := range want {
		if got[index] != statement {
			t.Errorf("statement %d = %+v, want %+v", index+1, got[index], statement)
		}
	}
}

// The display form the prompt templates read is unchanged by keeping the
// keyword apart: a modifier still reads "and <text>", the way it reads
// in the document.
func TestLoadStillFlattensModifiersForPrompts(t *testing.T) {
	t.Parallel()

	scenarios, err := NewRegistryLoader().Load(writeRegistry(t,
		"        - a thing\n        - and: another\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := scenarios[0].Steps.Given
	if len(got) != 2 || got[1] != "and another" {
		t.Errorf("given = %q, want the flattened modifier", got)
	}
}

// An unknown modifier key (anything but and/but) is refused here rather
// than reaching the generator, which would render it as `s.Foo(…)` — valid
// Go that fails to compile.
func TestLoadRefusesAnUnknownModifier(t *testing.T) {
	t.Parallel()

	_, err := NewRegistryLoader().Load(writeRegistry(t,
		"        - a thing\n        - foo: another\n"))

	if !errors.Is(err, ErrMalformedStepModifier) {
		t.Fatalf("want ErrMalformedStepModifier, got %v", err)
	}
}

// An empty key is refused by the same check rather than reaching a slice
// index that panics the whole CLI.
func TestLoadRefusesAnEmptyModifierKey(t *testing.T) {
	t.Parallel()

	_, err := NewRegistryLoader().Load(writeRegistry(t,
		"        - a thing\n        - \"\": another\n"))

	if !errors.Is(err, ErrMalformedStepModifier) {
		t.Fatalf("want ErrMalformedStepModifier, got %v", err)
	}
}

// A non-scalar value is refused too: node.Value is empty for a sequence, so
// reading it naively would silently set the step's text to "" — a step
// that binds to nothing and looks like the author wrote it.
func TestLoadRefusesANonScalarModifierValue(t *testing.T) {
	t.Parallel()

	for name, step := range map[string]string{
		"sequence": "        - and: [one, two]\n",
		"mapping":  "        - and: {nested: value}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRegistryLoader().Load(writeRegistry(t, "        - a thing\n"+step))

			if !errors.Is(err, ErrMalformedStepModifier) {
				t.Fatalf("want ErrMalformedStepModifier, got %v", err)
			}
		})
	}
}
