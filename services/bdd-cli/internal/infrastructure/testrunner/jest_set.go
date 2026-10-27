package testrunner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// RunSet reruns a Jest selection and requires each member's passing assertion.
func (r *JestRunner) RunSet(_ context.Context, members []*FailingTest) error {
	if len(members) == 0 {
		return nil
	}

	argv, err := jestSetArgv(members)
	if err != nil {
		return err
	}

	stdout, stderr, runErr := r.exec(CommandDir(members[0].RunnerConfig), PhaseRerun, argv)
	if runErr != nil && stdout.Len() == 0 {
		return fmt.Errorf("jest set rerun failed: %w (stderr: %s)", runErr, stderr.String())
	}

	report, err := parseJestReport(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("jest set report: %w", errors.Join(err, runErr))
	}

	if runErr != nil && len(jestReportToFailingTests(report, "", "")) == 0 {
		return runErr
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
		file := jestRepoRelative(workingDir(), result.Name)

		recordJestLoadOutcome(outcomes, file, result)

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

func (r *JestRunner) runOneLoad(ctx context.Context, member *FailingTest) (bool, string, error) {
	selected := *member

	err := r.RunSet(ctx, []*FailingTest{&selected})
	if err != nil {
		return false, "", err
	}

	return selected.LastRunPassed, selected.FailureOutput, nil
}

// A load member needs its file's successful result, even when it now contains assertions.
func jestLoadOutcome(result dto.JestTestResult) dtoOutcome {
	passed := result.Status == statusPassed
	for _, assertion := range result.AssertionResults {
		passed = passed && assertion.Status != statusFailed
	}

	return dtoOutcome{passed: passed, ran: true, output: result.Message}
}

// A load member has no assertion name: select all members' files in the mixed set.
func jestSetArgv(members []*FailingTest) ([]string, error) {
	hasLoad := false

	for _, member := range members {
		_, title, err := splitJestName(member.TestName)
		if err != nil {
			return nil, err
		}

		hasLoad = hasLoad || title == jestLoadMarker
	}

	if !hasLoad {
		return setArgv(members, "--testNamePattern", jestSetPattern)
	}

	argv, err := commandArgv(members[0])
	if err != nil {
		return nil, err
	}

	argv = append(argv, "--runTestsByPath")
	files := make(map[string]bool)

	for _, member := range members {
		file, _, _ := splitJestName(member.TestName)

		absolute, err := filepath.Abs(file)
		if err != nil {
			return nil, fmt.Errorf("jest file path: %w", err)
		}

		if !files[absolute] {
			argv = append(argv, absolute)
			files[absolute] = true
		}
	}

	return argv, nil
}

// Jest projects can report the same file more than once; every result must pass.
func recordJestLoadOutcome(outcomes map[string]dtoOutcome, file string, result dto.JestTestResult) {
	name := file + jestNameSeparator + jestLoadMarker

	outcome := jestLoadOutcome(result)
	if previous, exists := outcomes[name]; exists {
		outcome.passed = outcome.passed && previous.passed
		outcome.output = strings.TrimSpace(previous.output + "\n" + outcome.output)
	}

	outcomes[name] = outcome
}
