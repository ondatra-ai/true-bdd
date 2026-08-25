package gates_test

import (
	"os"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

const workflow = "../../.github/workflows/ci.yml"

// The invariant §6 of docs/for_further/task-automation.md is built on: the
// LIST of gates is single-sourced even though the SELECTION is local-only.
// This pair has drifted before — gates.sh ran lint-comments.sh, CI did not.
func TestCIRunsEveryGate(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("reading %s: %v", workflow, err)
	}

	pipeline := string(raw)

	for _, gate := range gates.All {
		if !strings.Contains(pipeline, "name: "+gate.Name) {
			t.Errorf("gate %q has no step named after it in %s", gate.Name, workflow)

			continue
		}

		marker := gate.CIAction
		if marker == "" {
			marker = gate.Command[0]
		}

		if !strings.Contains(pipeline, marker) {
			t.Errorf("gate %q: %s runs no %q", gate.Name, workflow, marker)
		}
	}
}

// The other direction: a step CI runs that the table does not know about is
// a gate the local pipeline never runs, which is the same drift mirrored.
func TestEveryCIGateIsInTheTable(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("reading %s: %v", workflow, err)
	}

	known := map[string]bool{}
	for _, gate := range gates.All {
		known[gate.Name] = true
	}

	// The install steps every gate depends on, which are setup rather than
	// checks and so have no row in the table.
	for _, setup := range []string{"Install yamale", "Install markdownlint-cli2", "Install alint"} {
		known[setup] = true
	}

	for _, line := range strings.Split(string(raw), "\n") {
		name, ok := stepName(line)
		if ok && !known[name] {
			t.Errorf("%s runs a step %q that the gate table does not carry", workflow, name)
		}
	}
}

// stepName pulls the name out of a `      - name: X` line, and reports false
// for anything else.
func stepName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- name:") {
		return "", false
	}

	return strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:")), true
}
