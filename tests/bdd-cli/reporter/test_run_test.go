package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeRunFixture lays out a minimal fixture dir: an engine log holding
// the supplied records plus the captured stream files they name.
func writeRunFixture(t *testing.T, records, stdout, stderr string) string {
	t.Helper()

	dir := t.TempDir()
	runDir := filepath.Join(dir, "tmp", "2026-01-01-00-00-00-1")

	err := os.MkdirAll(runDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		filepath.Join(dir, "tmp", "true-bdd.log.json"):        records,
		filepath.Join(runDir, "testrun-001-pw-discover.json"): stdout,
		filepath.Join(runDir, "testrun-001-pw-discover.txt"):  stderr,
	}

	for path, body := range files {
		writeErr := os.WriteFile(path, []byte(body), 0o644)
		if writeErr != nil {
			t.Fatal(writeErr)
		}
	}

	return dir
}

// TestTestRunsLoadsCapturedStreams is the reporter half of the evidence
// chain: the engine writes the exit record and the captured streams, and
// the report is only as good as its ability to find them again. It also
// pins the empty-stderr case, which must survive as a captured artifact
// rather than degrading into "not recorded".
func TestTestRunsLoadsCapturedStreams(t *testing.T) {
	t.Parallel()

	const report = `{"suites":[],"errors":[{"message":"webServer failed"}]}`

	records := `{"time":"2026-01-01T00:00:00.000+00:00","level":"DEBUG",` +
		`"msg":"Test runner returned","binary":"npx","dir":"tests",` +
		`"framework":"playwright","phase":"discover","exit_code":1,` +
		`"duration_ms":863,"stdout_bytes":54,"stderr_bytes":0,` +
		`"stdout_file":"tmp/2026-01-01-00-00-00-1/testrun-001-pw-discover.json",` +
		`"stderr_file":"tmp/2026-01-01-00-00-00-1/testrun-001-pw-discover.txt"}`

	dir := writeRunFixture(t, records, report, "")

	log, err := LoadEngineLog(filepath.Join(dir, "tmp", "true-bdd.log.json"))
	if err != nil {
		t.Fatalf("load log: %v", err)
	}

	runs := log.TestRuns(dir)
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}

	run := runs[0]

	if run.Label() != "playwright · discover" {
		t.Errorf("label = %q, want %q", run.Label(), "playwright · discover")
	}

	if !run.HasExit || run.ExitCode != 1 {
		t.Errorf("exit = %d (recorded=%v), want 1 (recorded)", run.ExitCode, run.HasExit)
	}

	if run.Duration != 863*time.Millisecond {
		t.Errorf("duration = %v, want 863ms", run.Duration)
	}

	if run.Stdout.Content != report {
		t.Errorf("stdout = %q, want the captured report", run.Stdout.Content)
	}

	if run.Stderr.Missing || run.Stderr.Bytes != 0 {
		t.Errorf("stderr = %d bytes (missing=%v), want a captured empty file",
			run.Stderr.Bytes, run.Stderr.Missing)
	}
}

// TestTestRunsReportsProcessThatNeverStarted covers the case the exit
// code alone cannot express: -1 plus a reason means no process ever ran,
// which must not read like a framework returning a failure.
func TestTestRunsReportsProcessThatNeverStarted(t *testing.T) {
	t.Parallel()

	records := `{"time":"2026-01-01T00:00:00.000+00:00","level":"DEBUG",` +
		`"msg":"Test runner returned","binary":"npx","framework":"jest",` +
		`"phase":"discover","exit_code":-1,"duration_ms":2,` +
		`"error":"exec: \"npx\": executable file not found in $PATH"}`

	dir := writeRunFixture(t, records, "", "")

	log, err := LoadEngineLog(filepath.Join(dir, "tmp", "true-bdd.log.json"))
	if err != nil {
		t.Fatalf("load log: %v", err)
	}

	runs := log.TestRuns(dir)
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}

	outcome := runs[0].Outcome()
	if want := "never started — exec: \"npx\": executable file not found in $PATH"; outcome != want {
		t.Errorf("outcome = %q, want %q", outcome, want)
	}

	if runs[0].Stdout.Path != "" {
		t.Errorf("stdout path = %q, want empty for an uncaptured stream", runs[0].Stdout.Path)
	}
}
