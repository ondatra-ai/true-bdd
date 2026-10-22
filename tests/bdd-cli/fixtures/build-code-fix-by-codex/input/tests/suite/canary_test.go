package suite_test

import (
	"os"
	"testing"
)

// TestCanaryRecordsEachSuiteRun always passes, so discovery never walks
// it and the fix loop never targets it. It appends one line every time
// the WHOLE suite runs.
//
// That is the assertion this fixture exists for. After a fix is applied,
// `build code` must re-run only the test it fixed, narrowed by name. A
// rerun that dropped the filter would execute this test a second time
// and leave two lines behind — a difference the golden tree catches
// byte-for-byte, in replay, with no model involved.
//
// Exit code alone cannot see it: an unfiltered rerun would also find the
// target passing and report success. Without this file the fixture would
// prove the fix loop converges, not that it narrowed.
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
