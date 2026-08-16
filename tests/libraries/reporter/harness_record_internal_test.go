package reporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// TestDiffLinesMatchLegacyFormat pins the rendered diff line against the
// string the harness used to print and the report used to scrape, so the
// page text does not change with the source.
func TestDiffLinesMatchLegacyFormat(t *testing.T) {
	lines := diffLines([]runner.FileChangeRecord{
		{Kind: "created", Path: "tests/integration/login.spec.ts", BytesAfter: 1841},
	})

	want := "created tests/integration/login.spec.ts (1841 bytes)"
	if len(lines) != 1 || lines[0] != want {
		t.Errorf("diffLines = %v, want [%q]", lines, want)
	}

	if diffLines(nil) != nil {
		t.Error("an empty diff must stay nil, so the page keeps saying 'none' rather than showing an empty list")
	}
}

// TestHarnessRecordFillsFixture pins the facts that exist only outside
// the engine's own process: the wall clock, the verdict and the diff.
func TestHarnessRecordFillsFixture(t *testing.T) {
	fixture := fixtureFromRecord(t, runner.HarnessRecord{
		Fixture: "fx",
		Verdict: runner.VerdictPass,
		WallMs:  289230,
		Diff:    []runner.FileChangeRecord{{Kind: "created", Path: "a.ts", BytesAfter: 12}},
	})

	if !fixture.HasWall || fixture.Wall != 289230*time.Millisecond {
		t.Errorf("wall = %v (has=%v), want 289.23s", fixture.Wall, fixture.HasWall)
	}

	// HasRecord is what lets the page say "the run changed no files"
	// instead of "the harness only prints a diff on failure".
	if !fixture.HasRecord {
		t.Error("HasRecord not set, so an empty diff would be described as unknown")
	}

	if fixture.Verdict != runner.VerdictPass {
		t.Errorf("verdict = %q, want PASS", fixture.Verdict)
	}

	if len(fixture.Diff) != 1 {
		t.Errorf("diff = %v, want one line", fixture.Diff)
	}
}

// TestJudgeCallUsesEndOfWindow pins that JudgeCall.At is the END of the
// judge window — phase.go measures the trailing harness block to that
// instant, so the near edge would leave the model call unaccounted for.
func TestJudgeCallUsesEndOfWindow(t *testing.T) {
	end := time.Now()

	fixture := fixtureFromRecord(t, runner.HarnessRecord{
		Fixture: "fx",
		Judge: runner.JudgeRecord{
			StartedAt: end.Add(-3 * time.Second),
			EndedAt:   end,
			CostUSD:   0.2866,
			Tokens:    69417,
		},
	})

	if fixture.Judge == nil {
		t.Fatal("judge not filled")
	}

	if !fixture.Judge.At.Equal(end) {
		t.Errorf("judge.At = %v, want the END of the window %v", fixture.Judge.At, end)
	}

	if fixture.Judge.CostUSD != 0.2866 || fixture.Judge.Tokens != 69417 {
		t.Errorf("judge spend = %v/%d, want 0.2866/69417",
			fixture.Judge.CostUSD, fixture.Judge.Tokens)
	}
}

// fixtureFromRecord writes a record and loads it back through the path
// the report uses.
func fixtureFromRecord(t *testing.T, record runner.HarnessRecord) *Fixture {
	t.Helper()

	dir := t.TempDir()
	writeRecord(t, dir, record)

	fixture := &Fixture{Name: record.Fixture}

	if !applyHarnessRecord(fixture, dir) {
		t.Fatal("applyHarnessRecord reported no record")
	}

	return fixture
}

// writeRecord puts a harness record where applyHarnessRecord looks.
func writeRecord(t *testing.T, dir string, record runner.HarnessRecord) {
	t.Helper()

	logDir := filepath.Join(dir, runner.SpawnLogDir)

	err := os.MkdirAll(logDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	blob, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	err = os.WriteFile(filepath.Join(logDir, runner.HarnessRecordFile), blob, 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
}
