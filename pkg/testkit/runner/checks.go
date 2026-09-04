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

// Verdict captures how one fixture was graded ABOVE its scenario. The
// judge rules in every mode; replay adds the golden byte-compare on top.
// The check that didn't run defaults true, so Pass requires both.
type Verdict struct {
	JudgeOK  bool
	GoldenOK bool
	JudgeMsg string
	Failures []string
	// JudgeStartedAt/JudgeEndedAt bracket the judge's model call — used to
	// pair a usage record with the fixture that caused it (see billJudge
	// in harness_record.go).
	JudgeStartedAt time.Time
	JudgeEndedAt   time.Time
	// The verbatim text of the judge call. Empty when the judge never got
	// as far as being asked. Persisted so a later comparison of two runs
	// can diff what the judge was told, not just what it concluded.
	JudgeSystemPrompt string
	JudgeUserPrompt   string
	JudgeResponse     string
	// JudgeModel is the pinned id the verdict was taken on.
	JudgeModel string
	// JudgeInputHash fingerprints the prompt the judge was shown, so two
	// runs can be told apart as judge noise versus harness drift.
	JudgeInputHash string
}

// Pass reports whether every check that ran was satisfied.
func (v Verdict) Pass() bool {
	return v.JudgeOK && v.GoldenOK
}

// EvaluateRecorded grades a replay run's WHEN-stage fidelity: every
// cassette consumed, and the resulting tree byte for byte. It asks no
// model — the judge grades the outcome separately, in every mode.
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

// Evaluate asks the judge to rule on a run. The call may take several
// seconds. The request is built by the caller, since what a judge rules
// on is the scenario's clauses, which the runner has no handle on.
func Evaluate(ctx context.Context, req JudgeRequest, judge Judge) Verdict {
	verdict := Verdict{GoldenOK: true}

	verdict.JudgeStartedAt = time.Now()

	outcome, err := judge.Verdict(ctx, req)

	// Stamped before the branch: a judge that errored may still have
	// burned tokens, and an unstamped window would bill them to nobody.
	verdict.JudgeEndedAt = time.Now()

	// Also carried before the branch, and for the same reason: an errored
	// or malformed call is precisely when the prompt is worth reading.
	verdict.JudgeSystemPrompt = outcome.SystemPrompt
	verdict.JudgeUserPrompt = outcome.UserPrompt
	verdict.JudgeResponse = outcome.Response
	verdict.JudgeModel = outcome.Model
	verdict.JudgeInputHash = outcome.InputHash

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
// never printed, with the tail of stdout riding along — a fixture is
// read long after the run, and "did not match" alone sends the reader hunting.
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
