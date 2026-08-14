package reportserver

import "testing"

// exit returns a pointer to an exit code, which TestSummary carries as
// *int so "never exited" is distinguishable from "exited 0".
func exit(code int) *int { return &code }

// Failure strings verbatim from runner/checks.go, so this file breaks if
// the harness changes its wording.
const (
	failKilled   = "exit code: got -1, want 0"
	failExitOne  = "exit code: got 1, want 0"
	failExitZero = "exit code: got 0, want 1"
	failStdout   = "stdout: 3 regex(es) did not match: " +
		"ALL CHECKS PASSED! | Story saved to: | REFINE COMPLETE"
	failJudgeBad = `judge: judge call errored: judge response did not match PASS or FAIL: ` +
		`<reason>: got="Looking at this task, I need to verify"`
)

// classifyCase is one observed failure shape and the reason it must
// reduce to.
type classifyCase struct {
	name    string
	summary TestSummary
	want    Outcome
	because string
}

// TestClassifyRealFailureShapes is table-driven over every failure shape
// actually present in tmp/test_run — the strings are verbatim from the
// harness, so this breaks if runner/checks.go changes its wording.
//
// The point of the ladder is that failures are multi-dimensional: a
// killed run also misses its stdout patterns and also fails the judge,
// because neither had anything coherent to work with. Classifying such a
// run by its downstream symptom would send someone to fix the assertions
// instead of the timeout.
func TestClassifyRealFailureShapes(t *testing.T) {
	cases := append(classifyCases(), classifyExitCases()...)
	cases = append(cases, classifyEdgeCases()...)

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := classify(testCase.summary)
			if got != testCase.want {
				t.Errorf("classify() = %q, want %q\n  %s", got, testCase.want, testCase.because)
			}
		})
	}
}

// classifyCases enumerates every failure shape present in tmp/test_run.
func classifyCases() []classifyCase {
	return []classifyCase{
		{
			name: "killed outranks the stdout misses it caused",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(-1),
				Failures: []string{
					failKilled,
					failStdout,
				},
			},
			want:    OutcomeKilled,
			because: "the process was killed; its stdout was never going to match",
		},
		{
			name: "killed outranks the judge rejection it caused",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(-1),
				Failures: []string{
					failKilled,
					"judge: the diff shows extensive tmp/ scratch artifacts but no file was created under docs/product/stories/",
				},
			},
			want:    OutcomeKilled,
			because: "a half-finished run leaves a half-finished diff for the judge",
		},
		{
			name: "killed with nothing else attached",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(-1),
				Failures: []string{failKilled},
			},
			want: OutcomeKilled,
		},
		{
			name: "the judge itself breaking is not a content rejection",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(0),
				Failures: []string{
					failJudgeBad,
				},
			},
			want:    OutcomeJudgeBroken,
			because: "the oracle failed, so the run is evidence of nothing about the product",
		},
	}
}

// classifyExitCases covers failures attributed to the command's own exit
// status, in both directions.
func classifyExitCases() []classifyCase {
	return []classifyCase{
		{
			name: "CLI exited non-zero when zero was wanted",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(1),
				Failures: []string{
					failExitOne,
					"judge: The diff shows no new file created under docs/product/stories/",
				},
			},
			want: OutcomeCrashed,
		},
		{
			name: "crashed outranks both downstream checks",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(1),
				Failures: []string{
					failExitOne,
					failStdout,
					"judge: The diff shows no changes to docs/product/stories/96.7-document-summary-step-qualifier.yaml",
				},
			},
			want: OutcomeCrashed,
		},
		{
			name: "a negative fixture that succeeded anyway",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(0),
				Failures: []string{
					failExitZero,
					`stdout: 1 regex(es) did not match: docs/product/product\.yaml`,
				},
			},
			want:    OutcomeUnexpectedSuccess,
			because: "expected a non-zero exit and got zero — the opposite direction from a crash",
		},
		{
			name: "ran fine, printed the wrong thing",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(0),
				Failures: []string{
					"stdout: 2 regex(es) did not match: Validation failed. | CREATE COMPLETE",
				},
			},
			want: OutcomeOutputMismatch,
		},
		{
			name: "ran fine, printed fine, judge rejected the content",
			summary: TestSummary{
				Verdict:  runnerVerdictFail,
				ExitCode: exit(0),
				Failures: []string{
					"judge: The only log entry in tmp/true-bdd.log.json is \"Loading full checklist\"",
				},
			},
			want: OutcomeContentRejected,
		},
	}
}

// classifyEdgeCases covers the outcomes that are not failures.
func classifyEdgeCases() []classifyCase {
	return []classifyCase{
		{
			name:    "a pass",
			summary: TestSummary{Verdict: runnerVerdictPass, ExitCode: exit(0)},
			want:    OutcomePass,
		},
		{
			name:    "a skip",
			summary: TestSummary{Verdict: runnerVerdictSkip},
			want:    OutcomeSkipped,
		},
		{
			name:    "no verdict at all",
			summary: TestSummary{},
			want:    OutcomeAbsent,
		},
		{
			name:    "a FAIL with no attributable check stays visible",
			summary: TestSummary{Verdict: runnerVerdictFail, ExitCode: exit(0)},
			want:    OutcomeCrashed,
		},
	}
}

// TestFailedAndObservedExcludeNonEvidence pins that a fixture which
// never ran is neither a pass nor a failure. Counting an absent cell as
// a pass would make a partial session look like a clean sweep.
func TestFailedAndObservedExcludeNonEvidence(t *testing.T) {
	for _, outcome := range []Outcome{OutcomeAbsent, OutcomeSkipped} {
		if outcome.Failed() {
			t.Errorf("%q counted as a failure", outcome)
		}

		if outcome.Observed() {
			t.Errorf("%q counted as evidence", outcome)
		}
	}

	if OutcomePass.Failed() {
		t.Error("a pass counted as a failure")
	}

	if !OutcomeKilled.Failed() || !OutcomeContentRejected.Failed() {
		t.Error("a real failure was not counted as one")
	}
}
