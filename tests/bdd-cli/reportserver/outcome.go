package reportserver

import "strings"

// Outcome is why one fixture run ended the way it did.
//
// The harness records a verdict of PASS or FAIL and a list of failure
// strings. FAIL alone is not actionable: a run killed at its timeout, a
// CLI that exited with an error, a run whose printed output was wrong,
// and a run whose content the judge rejected are four unrelated
// problems with four different owners, and they are all "FAIL" today.
type Outcome string

const (
	// OutcomePass is a fixture that satisfied every check.
	OutcomePass Outcome = "pass"
	// OutcomeKilled is a run that hit its timeout and was killed. The
	// harness reports this as exit code -1.
	OutcomeKilled Outcome = "killed"
	// OutcomeJudgeBroken is the judge itself failing — it answered with
	// prose instead of PASS or FAIL, so its verdict parsed to nothing.
	// Distinct from a rejection because the run yields no evidence about
	// the product either way.
	OutcomeJudgeBroken Outcome = "judge-broken"
	// OutcomeCrashed is the CLI exiting non-zero where zero was wanted.
	OutcomeCrashed Outcome = "crashed"
	// OutcomeUnexpectedSuccess is a negative fixture — one that expects a
	// non-zero exit — whose command succeeded anyway.
	OutcomeUnexpectedSuccess Outcome = "unexpected-success"
	// OutcomeOutputMismatch is a run that completed but whose stdout did
	// not match the patterns the fixture declared.
	OutcomeOutputMismatch Outcome = "output-mismatch"
	// OutcomeContentRejected is a run that completed and printed the right
	// things, but whose file changes the judge rejected against the rubric.
	OutcomeContentRejected Outcome = "content-rejected"
	// OutcomeSkipped is a fixture the suite skipped.
	OutcomeSkipped Outcome = "skipped"
	// OutcomeAbsent is a fixture that did not run in this session at all.
	// Never rendered as a pass: a partial session must read as partial.
	OutcomeAbsent Outcome = "absent"
)

// Failure-string prefixes the harness writes (runner/checks.go).
const (
	failurePrefixExit   = "exit code:"
	failurePrefixStdout = "stdout:"
	failurePrefixJudge  = "judge:"
	// judgeErroredMarker appears when ClaudeJudge could not parse a
	// verdict out of the model's reply at all.
	judgeErroredMarker = "judge call errored"
	// exitCodeKilled is what the harness records when the CLI never
	// exited on its own because the timeout killed it.
	exitCodeKilled = -1
)

// classify reduces one fixture's checks to a single primary reason.
func classify(summary TestSummary) Outcome {
	switch summary.Verdict {
	case runnerVerdictSkip:
		return OutcomeSkipped
	case runnerVerdictPass:
		return OutcomePass
	case runnerVerdictFail:
		return classifyFailure(summary)
	default:
		// No verdict recorded: the harness never finished this fixture.
		return OutcomeAbsent
	}
}

// classifyFailure walks the reasons a failed fixture could have failed,
// ordered by ROOT CAUSE, most upstream first.
//
// Failures are multi-dimensional — a killed run also misses its stdout
// patterns and also fails the judge, because neither had anything
// coherent to work with. Reporting such a run as "output mismatch" would
// send someone to fix the assertions instead of the timeout.
func classifyFailure(summary TestSummary) Outcome {
	// 1. Killed. Explains every downstream failure it drags with it.
	if summary.ExitCode != nil && *summary.ExitCode == exitCodeKilled {
		return OutcomeKilled
	}

	// 2. The oracle broke. Ranked above the product's own failures
	// because the verdict is not evidence about the product either way —
	// treating it as a content rejection would blame the code for the
	// judge's malformed reply.
	if hasFailure(summary.Failures, judgeErroredMarker) {
		return OutcomeJudgeBroken
	}

	// 3./4. The command's own exit status, in whichever direction.
	if hasPrefix(summary.Failures, failurePrefixExit) {
		if summary.ExitCode != nil && *summary.ExitCode == 0 {
			return OutcomeUnexpectedSuccess
		}

		return OutcomeCrashed
	}

	// 5. It ran and exited correctly, but printed the wrong thing.
	if hasPrefix(summary.Failures, failurePrefixStdout) {
		return OutcomeOutputMismatch
	}

	// 6. It ran, exited and printed correctly; the judge rejected what it
	// wrote to disk.
	if hasPrefix(summary.Failures, failurePrefixJudge) {
		return OutcomeContentRejected
	}

	// A FAIL the harness recorded without an attributable check. Reported
	// as a crash rather than invented into a category, so it stays visible.
	return OutcomeCrashed
}

// Failed reports whether an outcome represents a fixture that did not
// pass. Absent and skipped are neither passes nor failures — a fixture
// that never ran is not evidence.
func (o Outcome) Failed() bool {
	switch o {
	case OutcomePass, OutcomeSkipped, OutcomeAbsent:
		return false
	case OutcomeKilled, OutcomeJudgeBroken, OutcomeCrashed,
		OutcomeUnexpectedSuccess, OutcomeOutputMismatch, OutcomeContentRejected:
		return true
	default:
		return true
	}
}

// Observed reports whether the fixture actually ran, so counts are taken
// over evidence rather than over the size of the grid.
func (o Outcome) Observed() bool {
	return o != OutcomeAbsent && o != OutcomeSkipped
}

// outcomeLabels are the plain-English names shown to a reader.
// Deliberately not jargon: the page is read to decide what to go and fix.
func outcomeLabels() map[Outcome]string {
	return map[Outcome]string{
		OutcomePass:              "passed",
		OutcomeKilled:            "killed at the fixture timeout",
		OutcomeJudgeBroken:       "the judge itself broke",
		OutcomeCrashed:           "CLI exited with an error",
		OutcomeUnexpectedSuccess: "the command was supposed to fail and did not",
		OutcomeOutputMismatch:    "printed the wrong thing",
		OutcomeContentRejected:   "the judge rejected the result",
		OutcomeSkipped:           "skipped",
		OutcomeAbsent:            "did not run",
	}
}

// Label is how this outcome is described on the page.
func (o Outcome) Label() string {
	label, ok := outcomeLabels()[o]
	if !ok {
		return string(o)
	}

	return label
}

// outcomeReasons are the short forms written inside a grid cell, beside
// the duration. A pass carries none: green plus a number already says
// everything, and a word there would be noise on the majority of cells.
func outcomeReasons() map[Outcome]string {
	return map[Outcome]string{
		OutcomeKilled:      "timed out",
		OutcomeCrashed:     "CLI error",
		OutcomeJudgeBroken: "judge broke",
		// "no error raised" describes the COMMAND, which is what this
		// fixture asserts about. Saying "passed but should not have" read
		// as a claim about the test, which is the opposite of the truth:
		// the test is doing its job by going red here.
		OutcomeUnexpectedSuccess: "no error raised",
		OutcomeOutputMismatch:    "wrong output",
		OutcomeContentRejected:   "judge rejected",
		OutcomeSkipped:           "skipped",
	}
}

// Reason is the short phrase shown in a grid cell, empty for a pass or
// for a fixture that never ran.
func (o Outcome) Reason() string {
	return outcomeReasons()[o]
}

// hasPrefix reports whether any failure line starts with prefix.
func hasPrefix(failures []string, prefix string) bool {
	for _, failure := range failures {
		if strings.HasPrefix(failure, prefix) {
			return true
		}
	}

	return false
}

// hasFailure reports whether any failure line contains marker.
func hasFailure(failures []string, marker string) bool {
	for _, failure := range failures {
		if strings.Contains(failure, marker) {
			return true
		}
	}

	return false
}
