package testrunner

import (
	"regexp"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// startupReportJSON is Playwright 1.62's actual webServer-startup-failure
// report, captured verbatim: an empty suites[] with the run-level errors[]
// and stats[] blocks the engine used to discard.
const startupReportJSON = `{
  "suites": [],
  "errors": [
    { "message": "Error: Process from config.webServer was not able to start. Exit code: 1",
      "stack": "Error: Process from config.webServer was not able to start. Exit code: 1" }
  ],
  "stats": {
    "startTime": "2026-08-09T22:02:52.885Z",
    "duration": 509.408,
    "expected": 0, "skipped": 0, "unexpected": 0, "flaky": 0
  }
}`

// TestParsePlaywrightReportKeepsRunLevelEvidence covers the decode gap that
// hid a startup failure (see PlaywrightReport.Errors): only errors[] and
// stats.startTime attest that a run even started.
func TestParsePlaywrightReportKeepsRunLevelEvidence(t *testing.T) {
	t.Parallel()

	report, err := parsePlaywrightReport([]byte(startupReportJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(report.Errors) != 1 {
		t.Fatalf("run-level errors = %d, want 1", len(report.Errors))
	}

	want := "Error: Process from config.webServer was not able to start. Exit code: 1"
	if report.Errors[0].Message != want {
		t.Errorf("error message = %q, want %q", report.Errors[0].Message, want)
	}

	if report.Stats.StartTime == "" {
		t.Error("stats.startTime empty — nothing attests the run happened")
	}

	if report.Stats.Duration != 509.408 {
		t.Errorf("stats.duration = %v, want 509.408", report.Stats.Duration)
	}

	if msgs := playwrightErrorMessages(report.Errors); len(msgs) != 1 || msgs[0] != want {
		t.Errorf("flattened messages = %v, want [%q]", msgs, want)
	}
}

// TestParsePlaywrightReportPassingRun guards the other side of the
// distinction: a clean run has no run-level errors but still carries
// the stats block, so "ran and passed" stays separable from "never ran".
func TestParsePlaywrightReportPassingRun(t *testing.T) {
	t.Parallel()

	payload := `{"suites":[],"stats":{"startTime":"2026-08-09T22:02:52.885Z",` +
		`"duration":1200,"expected":4,"skipped":0,"unexpected":0,"flaky":0}}`

	report, err := parsePlaywrightReport([]byte(payload))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(report.Errors) != 0 {
		t.Errorf("run-level errors = %d, want 0", len(report.Errors))
	}

	if report.Stats.Expected != 4 {
		t.Errorf("stats.expected = %d, want 4", report.Stats.Expected)
	}
}

// Playwright's --grep matches the whole space-joined title path
// (Suite._grepTitleWithTags), not the report's " > "-joined chain: an
// anchored pattern matches nothing, silently reporting a fix as passed.
func TestPlaywrightGrepMatchesTheRealTitlePath(t *testing.T) {
	t.Parallel()

	chain := "integration/home.spec.ts > checkout > pays (fast)"
	grep := playwrightGrep(chain)

	pattern, err := regexp.Compile(grep)
	if err != nil {
		t.Fatalf("compile %q: %v", grep, err)
	}

	// What Playwright actually greps against: leading root title (""),
	// project, file, describes, title.
	actual := " chromium integration/home.spec.ts checkout pays (fast)"
	if !pattern.MatchString(actual) {
		t.Errorf("grep %q does not match Playwright's title path %q", grep, actual)
	}

	// The metacharacters in the title are escaped, not honoured.
	if pattern.MatchString(" chromium integration/home.spec.ts checkout pays fast") {
		t.Errorf("grep %q treated the title's parens as a regex group", grep)
	}

	// The anchored form this replaced could never match.
	anchored := regexp.MustCompile("^" + regexp.QuoteMeta(chain) + "$")
	if anchored.MatchString(actual) {
		t.Error("anchored chain unexpectedly matched; the regression this guards is gone")
	}
}

// A rerun that selected no test is not a passing test. Playwright emits
// its stats block either way, so the all-zero block is what separates
// them.
func TestPlaywrightRanNothing(t *testing.T) {
	t.Parallel()

	empty := &dto.PlaywrightReport{}
	if !playwrightRanNothing(empty) {
		t.Error("a report with no executed test must not read as a run")
	}

	ran := &dto.PlaywrightReport{Stats: dto.PlaywrightStats{Expected: 1}}
	if playwrightRanNothing(ran) {
		t.Error("a report with one expected test did run")
	}

	skipped := &dto.PlaywrightReport{Stats: dto.PlaywrightStats{Skipped: 1}}
	if playwrightRanNothing(skipped) {
		t.Error("a skipped test was still selected by the filter")
	}
}
