package runner

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Verdict captures the result of one fixture's three checks.
type Verdict struct {
	ExitOK   bool
	RegexOK  bool
	JudgeOK  bool
	JudgeMsg string
	Failures []string
}

// Pass reports whether all three checks were satisfied.
func (v Verdict) Pass() bool {
	return v.ExitOK && v.RegexOK && v.JudgeOK
}

// Evaluate runs the three checks (exit code, stdout regex, judge) and
// bundles the result. The judge call may take several seconds.
func Evaluate(ctx context.Context, fixture *Fixture, result *RunResult, judge Judge) Verdict {
	var verdict Verdict

	verdict.ExitOK = checkExitCode(result.ExitCode, fixture.ExpectedExitCode, &verdict.Failures)

	verdict.RegexOK = checkStdoutRegexes(result.Stdout, fixture.StdoutRegexes, &verdict.Failures)

	pass, reason, err := judge.Verdict(ctx, JudgeRequest{
		Cmd:       fixture.Cmd,
		JudgeSpec: fixture.JudgeSpec,
		Diff:      result.Diff,
	})
	if err != nil {
		verdict.JudgeOK = false
		verdict.JudgeMsg = "judge call errored: " + err.Error()
		verdict.Failures = append(verdict.Failures, "judge: "+verdict.JudgeMsg)

		return verdict
	}

	verdict.JudgeOK = pass
	verdict.JudgeMsg = reason

	if !pass {
		verdict.Failures = append(verdict.Failures, "judge: "+reason)
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
