package gates_test

import (
	"os"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

const workflow = "../../.github/workflows/ci.yml"

// readWorkflow returns ci.yml, which both guards below read.
func readWorkflow(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("reading %s: %v", workflow, err)
	}

	return string(raw)
}

// CI runs the pipeline through the runner rather than a copy of it. The list
// and CI drifted before — the local pipeline ran the comment gate and CI did
// not, for months — and one invocation is what makes that impossible.
func TestCIRunsTheGateRunner(t *testing.T) {
	t.Parallel()

	if !strings.Contains(readWorkflow(t), "./scripts/cmd/gates run") {
		t.Errorf("%s does not run ./scripts/cmd/gates", workflow)
	}
}

// The other direction, and the one that keeps the single source single: a CI
// step NAMED after a gate is somebody re-listing the table by hand.
func TestCIDoesNotHandListGates(t *testing.T) {
	t.Parallel()

	for _, line := range strings.Split(readWorkflow(t), "\n") {
		name, ok := stepName(line)
		if !ok {
			continue
		}

		for _, gate := range gates.All {
			if name == gate.Name {
				t.Errorf("%s has a step %q — gates belong to the table, not to CI",
					workflow, name)
			}
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
