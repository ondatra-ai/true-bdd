package testrunner

import (
	"fmt"
	"strings"
	"time"
)

// Layer constants used by FailingTest.Layer. Match the keys under
// architecture.yaml's `quality_gate.tests:` block.
const (
	LayerUnit        = "unit"
	LayerIntegration = "integration"
	LayerE2E         = "e2e"
)

// FailureOutputCap is the maximum number of bytes of failure output
// retained on a FailingTest. Output is tail-truncated so the most
// recent failure context survives.
const FailureOutputCap = 8 * 1024

// FailingTest is one engine item walked by `build code`. Produced by a
// Runner's Discover; refreshed in-place by RunOne during the engine's
// PostFix hook.
type FailingTest struct {
	ID            string    // "<service>/<layer>/<sanitized framework id>"
	Service       string    // architecture.yaml service.name
	Layer         string    // LayerUnit | LayerIntegration | LayerE2E
	Framework     string    // "go-test" | "playwright" | "jest"
	TestName      string    // framework-native re-run identifier
	FilePath      string    // repo-relative path of the test source
	FailureOutput string    // tail of stdout/stderr from last run (cap 8 KB)
	LastRunPassed bool      // updated by RunOne in PostFix
	LastRunAt     time.Time // updated by RunOne in PostFix
	// RunnerConfig captures the original architecture.yaml block that
	// produced this test so RunOne can re-invoke the framework with the
	// same config file and root path. Populated by Discover; not part of
	// the prompt-visible subject.
	RunnerConfig Config
}

// Subject is the GetSubject implementation for the build-code engine.
// Returns (id, testName) for the report-builder header and tmp-file
// naming.
func Subject(item *FailingTest) (string, string) {
	return item.ID, item.TestName
}

// BuildID assembles a deterministic, filename-safe id from the four
// inputs that together uniquely identify a failing test in a build-code
// walk. Slashes inside the framework-native id are folded to `.` so the
// result can be spliced into tmp file paths without escaping.
func BuildID(service, layer, framework, frameworkID string) string {
	safe := strings.ReplaceAll(frameworkID, "/", ".")
	safe = strings.ReplaceAll(safe, " ", "_")

	return service + "/" + layer + "/" + framework + ":" + safe
}

// TruncateTail returns the last `maxBytes` bytes of `payload`, leaving
// a marker when truncation occurred. Used for FailingTest.FailureOutput
// so prompts stay within token budgets when test output is verbose.
func TruncateTail(payload string, maxBytes int) string {
	if len(payload) <= maxBytes {
		return payload
	}

	return fmt.Sprintf("... [truncated %d bytes] ...\n%s",
		len(payload)-maxBytes, payload[len(payload)-maxBytes:])
}
