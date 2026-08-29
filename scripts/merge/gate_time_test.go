package merge_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

// TestGateTimeIsPulledButNotAStep pins both halves of the CI gate-time pull:
// the numbers it reads out of the Actions API, and its absence from the report.
func TestGateTimeIsPulledButNotAStep(t *testing.T) {
	t.Parallel()

	folded, ok := merge.GateTimings([]byte(readFixture(t, "ci-jobs.json")))
	if !ok {
		t.Fatal("the gates job was not found in the jobs payload")
	}

	t.Run("timings", func(t *testing.T) {
		t.Parallel()

		if want := 3*time.Minute + 6*time.Second; folded.Total != want {
			t.Errorf("job total is %s, want %s", folded.Total, want)
		}

		if !slices.Contains(folded.PerGate, "BDD fixtures (replay) 99s") {
			t.Errorf("replay gate reads %v", folded.PerGate)
		}

		assertBreakdownIsTheGateTable(t, folded.PerGate)
	})

	t.Run("not a step", func(t *testing.T) {
		t.Parallel()

		assertNoReportKeys(t, folded.Attrs)
	})
}

// assertBreakdownIsTheGateTable checks every gate is timed and nothing else is.
func assertBreakdownIsTheGateTable(t *testing.T, breakdown []string) {
	t.Helper()

	for _, gate := range gates.All {
		if !slices.ContainsFunc(breakdown, hasPrefix(gate.Name+" ")) {
			t.Errorf("gate %q is missing from the breakdown", gate.Name)
		}
	}

	for _, step := range []string{"Set up job", "Post Lint", "Install alint"} {
		if slices.ContainsFunc(breakdown, hasPrefix(step+" ")) {
			t.Errorf("%q is a CI step, not a gate — it must not be timed here", step)
		}
	}
}

// assertNoReportKeys is the requirement, as a check: a record carrying `tree`
// or `duration_ms` becomes a step in scripts/report, and `run` collides with
// the process id pkg/logging stamps on every record.
func assertNoReportKeys(t *testing.T, attrs []any) {
	t.Helper()

	forbidden := []string{"tree", "duration_ms", "run"}

	for index := 0; index < len(attrs); index += 2 {
		key, isString := attrs[index].(string)
		if !isString {
			t.Fatalf("attribute %d is not a string key: %v", index, attrs[index])
		}

		if slices.Contains(forbidden, key) {
			t.Errorf("attribute %q makes the record a step in scripts/report", key)
		}
	}
}

func hasPrefix(prefix string) func(string) bool {
	return func(row string) bool { return strings.HasPrefix(row, prefix) }
}
