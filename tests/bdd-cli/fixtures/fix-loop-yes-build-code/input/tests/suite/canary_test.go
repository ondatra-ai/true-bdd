package suite_test

import (
	"os"
	"testing"
)

// TestCanaryRecordsEachSuiteRun records full-suite runs, including the final
// outer walk. An isolated rerun must not execute it; a package-build rerun
// has no test name to filter and executes it once the package compiles.
func TestCanaryRecordsEachSuiteRun(t *testing.T) {
	file, err := os.OpenFile("suite-runs.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open canary log: %v", err)
	}

	defer func() { _ = file.Close() }()

	_, err = file.WriteString("suite ran\n")
	if err != nil {
		t.Fatalf("write canary log: %v", err)
	}
}
