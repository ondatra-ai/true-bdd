package testrunner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner/dto"
)

// RunSet reruns a Playwright selection, or the unfiltered suite for startup members.
func (r *PlaywrightRunner) RunSet(_ context.Context, members []*FailingTest) error {
	if len(members) == 0 {
		return nil
	}

	argv, err := setArgv(members, "--grep", playwrightSetPattern)
	if err != nil {
		return err
	}

	stdout, stderr, runErr := r.exec(CommandDir(members[0].RunnerConfig), PhaseRerun, argv)
	if runErr != nil && stdout.Len() == 0 {
		return fmt.Errorf("playwright set rerun failed: %w (stderr: %s)", runErr, stderr.String())
	}

	report, err := parsePlaywrightReport(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("playwright set report: %w", errors.Join(err, runErr))
	}

	if runErr != nil && !playwrightHasStartup(members) && len(playwrightReportToFailingTests(report, "", "")) == 0 {
		return runErr
	}

	outcomes := make(map[string]dtoOutcome)
	for _, suite := range report.Suites {
		playwrightSetOutcomes(outcomes, suite, "", suite.File)
	}

	outcomes[playwrightStartupMarker] = playwrightStartupOutcome(report, runErr, stderr.String(), stdout.String())
	recordOutcomes(members, outcomes)

	return nil
}

func playwrightSetPattern(name string) (string, bool, error) {
	if name == playwrightStartupMarker {
		return "", true, nil
	}

	file, title, err := splitPlaywrightName(name)
	if err != nil || file == "" {
		return "", false, err
	}

	return playwrightGrep(title), false, nil
}

func playwrightSetOutcomes(outcomes map[string]dtoOutcome, suite dto.PlaywrightSuite, chain, file string) {
	chain = joinTitle(chain, suite.Title)

	file = pickFile(file, suite.File)
	for _, spec := range suite.Specs {
		passed := len(spec.Tests) > 0

		var output strings.Builder

		for _, test := range spec.Tests {
			passed = passed && playwrightTestPassed(test)
			output.WriteString(formatPlaywrightFailure(test.Results))
		}

		name := pickFile(file, spec.File) + playwrightNameSeparator + joinTitle(chain, spec.Title)
		outcomes[name] = dtoOutcome{passed: passed, ran: playwrightSpecRan(spec), output: output.String()}
	}

	for _, child := range suite.Suites {
		playwrightSetOutcomes(outcomes, child, chain, file)
	}
}

func playwrightTestPassed(test dto.PlaywrightTest) bool {
	return len(test.Results) > 0 && test.Results[len(test.Results)-1].Status == statusPassed
}

func playwrightStartupOutcome(report *dto.PlaywrightReport, runErr error, stderr, stdout string) dtoOutcome {
	passed := runErr == nil && len(report.Errors) == 0 && !playwrightRanNothing(report)

	output := ""
	if !passed {
		output = stderr + stdout
	}

	return dtoOutcome{passed: passed, ran: true, output: output}
}

func playwrightSpecRan(spec dto.PlaywrightSpec) bool {
	for _, test := range spec.Tests {
		if len(test.Results) == 0 {
			return false
		}

		if test.Results[len(test.Results)-1].Status == "skipped" {
			return false
		}
	}

	return len(spec.Tests) > 0
}

func playwrightHasStartup(members []*FailingTest) bool {
	for _, member := range members {
		if member.TestName == playwrightStartupMarker {
			return true
		}
	}

	return false
}
