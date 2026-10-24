package testrunner

import (
	"context"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// RunSet reruns a Jest selection and requires each member's passing assertion.
func (r *JestRunner) RunSet(_ context.Context, members []*FailingTest) error {
	if len(members) == 0 {
		return nil
	}

	argv, err := setArgv(members, "--testNamePattern", jestSetPattern)
	if err != nil {
		return err
	}

	stdout, stderr, runErr := r.exec(CommandDir(members[0].RunnerConfig), PhaseRerun, argv)
	if runErr != nil && stdout.Len() == 0 {
		return fmt.Errorf("jest set rerun failed: %w (stderr: %s)", runErr, stderr.String())
	}

	report, err := parseJestReport(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("jest set report: %w", err)
	}

	recordOutcomes(members, jestSetOutcomes(report))

	return nil
}

// dtoOutcome is a decoded native result, before it is attached to a selected member.
type dtoOutcome struct {
	passed bool
	ran    bool
	output string
}

func jestSetPattern(name string) (string, bool, error) {
	file, title, err := splitJestName(name)
	if err != nil || file == "" {
		return "", false, err
	}

	return "^" + jestRegexMeta.ReplaceAllString(title, `\$0`) + "$", false, nil
}

func jestSetOutcomes(report *dto.JestReport) map[string]dtoOutcome {
	outcomes := make(map[string]dtoOutcome)

	for _, result := range report.TestResults {
		for _, assertion := range result.AssertionResults {
			name := jestRepoRelative(workingDir(), result.Name) + jestNameSeparator + assertion.FullName
			outcomes[name] = dtoOutcome{
				passed: assertion.Status == statusPassed,
				ran:    assertion.Status == statusPassed || assertion.Status == statusFailed,
				output: strings.Join(assertion.FailureMessages, "\n"),
			}
		}
	}

	return outcomes
}
