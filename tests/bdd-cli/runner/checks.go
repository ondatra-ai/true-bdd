package runner

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Verdict captures the result of one fixture's checks.
//
// A run is graded one of two ways, never both. Live and record call the
// JUDGE, because the model output is new and only a reader can say
// whether it satisfies the rubric. Replay compares the recorded OUTCOME,
// because every AI-written file was materialised from a cassette — the
// only thing that can have moved is the engine, and a byte comparison
// says so exactly, for free, and the same way every time. Whichever
// check did not run is left true; Pass requires both.
type Verdict struct {
	ExitOK   bool
	RegexOK  bool
	JudgeOK  bool
	GoldenOK bool
	JudgeMsg string
	Failures []string
	// JudgeStartedAt/JudgeEndedAt bracket the judge's model call. The
	// run report measures the trailing harness block from the far edge —
	// it is the only stamp that exists after the engine's log goes
	// quiet — and the window is what pairs a usage record with the
	// fixture that caused it.
	JudgeStartedAt time.Time
	JudgeEndedAt   time.Time
	// The verbatim text of the judge call. Empty when the judge never
	// got as far as being asked. Persisted by the recorder so a later
	// comparison of two runs can diff what the judge was told, not just
	// what it concluded.
	JudgeSystemPrompt string
	JudgeUserPrompt   string
	JudgeResponse     string
}

// Pass reports whether every check that ran was satisfied.
func (v Verdict) Pass() bool {
	return v.ExitOK && v.RegexOK && v.JudgeOK && v.GoldenOK
}

// EvaluateRecorded grades a replay run against its recording: exit code,
// stdout regexes, every cassette consumed, and the resulting tree.
//
// No model is asked anything. That is the point — a replay verdict that
// depended on a model would inherit the model's variance, and this suite
// has already watched one grade the same bytes PASS, FAIL and PASS.
func EvaluateRecorded(
	fixture *Fixture,
	result *RunResult,
	golden *GoldenTree,
	cassettesDir, stateDir string,
) Verdict {
	verdict := Verdict{JudgeOK: true}

	verdict.ExitOK = checkExitCode(result.ExitCode, fixture.ExpectedExitCode, &verdict.Failures)

	verdict.RegexOK = checkStdoutRegexes(result.Stdout, fixture.StdoutRegexes, &verdict.Failures)

	failures := CheckCassettesConsumed(cassettesDir, stateDir)
	failures = append(failures, CompareGolden(golden, result.Diff)...)

	verdict.GoldenOK = len(failures) == 0
	verdict.Failures = append(verdict.Failures, failures...)

	if !verdict.GoldenOK {
		verdict.JudgeMsg = fmt.Sprintf("%d difference(s) from the recording", len(failures))
	}

	return verdict
}

// Evaluate runs the three checks (exit code, stdout regex, judge) and
// bundles the result. The judge call may take several seconds.
func Evaluate(ctx context.Context, fixture *Fixture, result *RunResult, judge Judge) Verdict {
	verdict := Verdict{GoldenOK: true}

	verdict.ExitOK = checkExitCode(result.ExitCode, fixture.ExpectedExitCode, &verdict.Failures)

	verdict.RegexOK = checkStdoutRegexes(result.Stdout, fixture.StdoutRegexes, &verdict.Failures)

	verdict.JudgeStartedAt = time.Now()

	outcome, err := judge.Verdict(ctx, JudgeRequest{
		Cmd:       fixture.Cmd,
		JudgeSpec: fixture.JudgeSpec,
		Diff:      result.Diff,
	})

	// Stamped before the branch: a judge that errored may still have
	// burned tokens, and an unstamped window would bill them to nobody.
	verdict.JudgeEndedAt = time.Now()

	// Also carried before the branch, and for the same reason: an errored
	// or malformed call is precisely when the prompt is worth reading.
	verdict.JudgeSystemPrompt = outcome.SystemPrompt
	verdict.JudgeUserPrompt = outcome.UserPrompt
	verdict.JudgeResponse = outcome.Response

	if err != nil {
		verdict.JudgeOK = false
		verdict.JudgeMsg = "judge call errored: " + err.Error()
		verdict.Failures = append(verdict.Failures, "judge: "+verdict.JudgeMsg)

		return verdict
	}

	verdict.JudgeOK = outcome.Pass
	verdict.JudgeMsg = outcome.Reason

	if !outcome.Pass {
		verdict.Failures = append(verdict.Failures, "judge: "+outcome.Reason)
	}

	return verdict
}

func checkExitCode(actual, expected int, failures *[]string) bool {
	if actual == expected {
		return true
	}

	*failures = append(*failures, fmt.Sprintf(
		"exit code: got %d, want %d", actual, expected,
	))

	return false
}

func checkStdoutRegexes(stdout string, regexes []*regexp.Regexp, failures *[]string) bool {
	if len(regexes) == 0 {
		return true
	}

	var missing []string

	for _, re := range regexes {
		if !re.MatchString(stdout) {
			missing = append(missing, re.String())
		}
	}

	if len(missing) == 0 {
		return true
	}

	*failures = append(*failures, fmt.Sprintf(
		"stdout: %d regex(es) did not match: %s",
		len(missing), strings.Join(missing, " | "),
	))

	return false
}
