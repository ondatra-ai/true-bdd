package testkit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func TestVerdictPassReportsAPassingTestWithoutAJudge(t *testing.T) {
	t.Parallel()
	assertReportedVerdict(t, "pass", VerdictPass)
}

func TestVerdictFailReportsAFailingTestWithoutAJudge(t *testing.T) {
	t.Parallel()
	assertReportedVerdict(t, "fail", VerdictFail)
}

func TestVerdictSkipReportsASkippedTestWithoutAJudge(t *testing.T) {
	t.Parallel()
	assertReportedVerdict(t, "skip", VerdictSkip)
}

func assertReportedVerdict(t *testing.T, outcome, want string) {
	t.Helper()
	session := t.TempDir()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestReportedVerdictProcess$")

	command.Env = append(os.Environ(), "TRUE_BDD_TEST_OUTCOME="+outcome, "TRUE_BDD_TEST_SESSION="+session)

	output, err := command.CombinedOutput()
	if (err != nil) != (outcome == "fail") {
		t.Fatalf("test process: %v\n%s", err, output)
	}

	data, err := Read(filepath.Join(session, snapshotFixtureName, SpawnLogDir, HarnessRecordFile))
	if err != nil {
		t.Fatal(err)
	}

	var record HarnessRecord

	err = json.Unmarshal(data, &record)
	if err != nil {
		t.Fatal(err)
	}

	if record.Verdict != want || !record.Judge.StartedAt.IsZero() {
		t.Fatalf("test outcome must be reported without a semantic verdict: %+v", record)
	}
}

func TestReportedVerdictProcess(t *testing.T) {
	outcome := os.Getenv("TRUE_BDD_TEST_OUTCOME")
	if outcome == "" {
		return
	}

	_, err := newScenarioRun(&harness{SessionRoot: os.Getenv("TRUE_BDD_TEST_SESSION")})(
		&bddgo.World{T: t, Scenario: Scenario{ID: snapshotFixtureName}})
	if err != nil {
		t.Fatal(err)
	}

	switch outcome {
	case "fail":
		t.Error("deliberate test failure")
	case "skip":
		t.Skip("deliberate test skip")
	case "pass":
		return
	default:
		t.Fatalf("unknown test outcome %q", outcome)
	}
}
