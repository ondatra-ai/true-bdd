package bddgo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// webSuite is the suite these tests make answer.
const webSuite = "bdd-web"

// Named after the SUITE, so one `go test ./tests/...` may start several
// suites and no answer lands on top of another. The directory is created
// rather than assumed.
func TestWriteCoverageReportNamesTheFileAfterTheSuite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created-yet")
	t.Setenv(coverageReportEnv, dir)

	writeCoverageReport(t, coverageReport{Schema: coverageSchema, Suite: webSuite})

	data, err := os.ReadFile(filepath.Join(dir, webSuite+".json"))
	if err != nil {
		t.Fatalf("read the report the suite should have written: %v", err)
	}

	var written coverageReport

	err = json.Unmarshal(data, &written)
	if err != nil {
		t.Fatalf("parse the written report: %v", err)
	}

	if written.Suite != webSuite {
		t.Errorf("report suite = %q, want %q", written.Suite, webSuite)
	}
}

// An unset variable means a person is running the test, and a person
// reads the failures — nothing is written anywhere.
func TestWriteCoverageReportWritesNothingWithoutTheVariable(t *testing.T) {
	dir := t.TempDir()

	t.Setenv(coverageReportEnv, "")
	t.Chdir(dir)

	writeCoverageReport(t, coverageReport{Schema: coverageSchema, Suite: webSuite})

	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("list the working directory: %v", err)
	}

	if len(left) != 0 {
		t.Errorf("wrote %d file(s) with no report directory asked for", len(left))
	}
}
