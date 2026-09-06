package gates_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

const registry = "docs/scenarios.yaml"

func names(selected []gates.Gate) []string {
	out := make([]string, 0, len(selected))
	for _, gate := range selected {
		out = append(out, gate.Name)
	}

	return out
}

// Every glob in the table must be a shape MatchGlob understands. An
// unsupported one matches nothing and silently widens the selection.
func TestEveryGlobIsSupported(t *testing.T) {
	t.Parallel()

	for _, gate := range gates.All {
		for _, glob := range gate.Globs {
			if !gates.SupportedGlob(glob) {
				t.Errorf("%s: glob %q is a shape MatchGlob does not handle", gate.Name, glob)
			}
		}
	}
}

// The documentation ticket is what the whole selector is for: ~2s of gates
// instead of ~140s.
// lintGate is the one row every lint leaf now lives behind.
const lintGate = "Lint"

func TestDocumentationDiffSkipsTheExpensiveGates(t *testing.T) {
	t.Parallel()

	got := names(gates.Select([]string{"docs/for_further/observability.md", "README.md"}))
	want := []string{lintGate}

	if !slices.Equal(got, want) {
		t.Errorf("Select(docs only) = %v, want %v", got, want)
	}
}

func TestScenarioRegistryPullsTheBDDGates(t *testing.T) {
	t.Parallel()

	got := names(gates.Select([]string{registry}))

	for _, want := range []string{"BDD cli coverage guards", lintGate} {
		if !slices.Contains(got, want) {
			t.Errorf("a diff to the registry did not select %q; got %v", want, got)
		}
	}
}

// Fail-safe. A path under a directory no rule mentions runs everything —
// silence here is how a new tree slips past every check.
func TestUnknownPathRunsEverything(t *testing.T) {
	t.Parallel()

	got := gates.Select([]string{"docs/for_further/x.md", "brand-new-tree/thing.txt"})
	if len(got) != len(gates.All) {
		t.Errorf("Select(unknown path) = %d gates, want all %d", len(got), len(gates.All))
	}
}

func TestEmptyDiffRunsEverything(t *testing.T) {
	t.Parallel()

	if got := gates.Select(nil); len(got) != len(gates.All) {
		t.Errorf("Select(nil) = %d gates, want all %d", len(got), len(gates.All))
	}
}

func TestMatchGlob(t *testing.T) {
	t.Parallel()

	cases := []struct {
		glob, path string
		want       bool
	}{
		{"**/*.go", "scripts/gates/table.go", true},
		{"**/*.go", "scripts/gates/table.yaml", false},
		{"true-bdd/**", "true-bdd/checklists/us-create.yaml", true},
		{"true-bdd/**", "true-bdd.yaml", false},
		{registry, registry, true},
		{registry, registry + ".bak", false},
	}

	for _, tc := range cases {
		if got := gates.MatchGlob(tc.glob, tc.path); got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.glob, tc.path, got, tc.want)
		}
	}
}
