package runner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// ErrExitCode and ErrStdoutUnmatched are the two assertions a scenario's
// Then steps make on every run, in every mode.
var (
	ErrExitCode        = errors.New("exit code")
	ErrStdoutUnmatched = errors.New("stdout matched no line")
)

// stdoutTailBytes caps how much output an unmatched-pattern failure
// quotes back. Enough to see what the run did print instead.
const stdoutTailBytes = 2048

// Verdict captures how one fixture was graded ABOVE its scenario.
//
// The scenario's own Then steps already asserted the exit code and the
// stdout patterns — on every run, in every mode, by the suite's step
// definitions. What is left here is the part no step can state: whether
// the files a run produced still match what was recorded, and whether a
// new model response satisfies the rubric.
//
// A run gets one of those two, never both. Live and record call the
// JUDGE, because the output is new and only a reader can say whether it
// satisfies the rubric. Replay compares the recorded OUTCOME, because
// every AI-written file was materialised from a cassette — the only
// thing that can have moved is the engine, and a byte comparison says so
// exactly, for free, and the same way every time. Whichever check did
// not run is left true; Pass requires both.
type Verdict struct {
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
	return v.JudgeOK && v.GoldenOK
}

// EvaluateRecorded grades a replay run against its recording: every
// cassette consumed, and the resulting tree byte for byte.
//
// No model is asked anything. That is the point — a replay verdict that
// depended on a model would inherit the model's variance, and this suite
// has already watched one grade the same bytes PASS, FAIL and PASS.
func EvaluateRecorded(
	result *RunResult,
	golden *GoldenTree,
	cassettesDir, stateDir string,
) Verdict {
	verdict := Verdict{JudgeOK: true}

	failures := CheckCassettesConsumed(cassettesDir, stateDir)
	failures = append(failures, CompareGolden(golden, result.Diff)...)

	verdict.GoldenOK = len(failures) == 0
	verdict.Failures = append(verdict.Failures, failures...)

	if !verdict.GoldenOK {
		verdict.JudgeMsg = fmt.Sprintf("%d difference(s) from the recording", len(failures))
	}

	return verdict
}

// Evaluate asks the judge whether the run's diff satisfies the
// fixture's rubric. The call may take several seconds.
func Evaluate(ctx context.Context, fixture *Fixture, result *RunResult, judge Judge) Verdict {
	verdict := Verdict{GoldenOK: true}

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

// CheckExitCode reports the failure line for a run that exited with
// something other than what its scenario asked for.
func CheckExitCode(actual, expected int) error {
	if actual == expected {
		return nil
	}

	return fmt.Errorf("%w: got %d, want %d", ErrExitCode, actual, expected)
}

// CheckStdout reports the failure line for a pattern the run's stdout
// never printed. The tail of stdout rides along: a fixture is read long
// after the run, and "did not match" without the output beside it sends
// the reader to a file they have to go find.
func CheckStdout(stdout string, pattern *regexp.Regexp) error {
	if pattern.MatchString(stdout) {
		return nil
	}

	return fmt.Errorf("%w: %s\n--- last %d bytes of stdout ---\n%s",
		ErrStdoutUnmatched, pattern.String(), stdoutTailBytes, stdoutTail(stdout))
}

func stdoutTail(stdout string) string {
	if len(stdout) <= stdoutTailBytes {
		return stdout
	}

	return "…" + stdout[len(stdout)-stdoutTailBytes:]
}
