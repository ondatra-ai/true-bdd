package testrunner

import (
	"strings"
	"time"
)

// FailingTest is one engine item walked by `build code`. Produced by a
// Runner's Discover; refreshed in-place by RunOne during the engine's
// PostFix hook.
type FailingTest struct {
	ID            string    // "<service>/<suite>/<sanitized framework id>"
	Service       string    // architecture.yaml service.name — the suite's `service:`
	Suite         string    // architecture.yaml testing.suites[].name
	Framework     string    // "go-test" | "playwright" | "jest"
	TestName      string    // framework-native re-run identifier
	FilePath      string    // repo-relative path of the test source
	FailureOutput string    // complete stdout/stderr from last run
	LastRunPassed bool      // updated by RunOne in PostFix
	LastRunAt     time.Time // updated by RunOne in PostFix
	// RunnerConfig captures the original architecture.yaml block that
	// produced this test, so RunOne can re-invoke the framework with the
	// same config file and root path; populated by Discover, not prompt-visible.
	RunnerConfig Config
}

// Subject is the GetSubject implementation for the build-code engine.
// Returns (id, testName) for the report-builder header and tmp-file
// naming.
func Subject(item *FailingTest) (string, string) {
	return item.ID, item.TestName
}

// BuildID assembles a deterministic, filename-safe id from the four inputs
// that identify a failing test; slashes in the framework-native id are
// folded to `.` so the id can be spliced into tmp file paths unescaped.
func BuildID(service, suite, framework, frameworkID string) string {
	safe := strings.ReplaceAll(frameworkID, "/", ".")
	safe = strings.ReplaceAll(safe, " ", "_")

	return service + "/" + suite + "/" + framework + ":" + safe
}
