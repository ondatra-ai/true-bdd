package suite_test

import (
	"os"
	"testing"
)

// TestCanaryRecordsEachSuiteRun always passes once the package compiles,
// and appends one line every time the package's tests run.
//
// That is the assertion this fixture exists for. A package that did not
// compile is re-run after its fix without a test filter. A rerun narrowed
// by a name filter would match no test at all: the package would compile,
// run nothing and still pass, leaving no line here. The unfiltered rerun
// leaves exactly one, which the golden tree pins byte for byte in replay.
//
// The file lands next to this package because `go test` runs a test
// binary with its own package directory as the working directory.
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
