package testrunner

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli"
)

// RedRun separates executed step failures from green and invalid Scenario results.
type RedRun struct {
	Red     []string
	Green   []string
	Invalid []string
}

// AssessRedRun requires an owning test's failure and the harness's executed-step diagnostic.
func AssessRedRun(payload []byte, identities []string) RedRun {
	events := parseGoTestEvents(payload)
	stepFailure := regexp.MustCompile(`(?m)^\s+step [1-9][0-9]*/[1-9][0-9]* failed: (Given|When|Then|And|But) .+`)
	verdict := RedRun{}

	for _, identity := range identities {
		pkg, name, err := splitGoTestName(identity)
		if err != nil {
			verdict.Invalid = append(verdict.Invalid, identity+": "+err.Error())

			continue
		}

		key := pkg + "\x00" + name
		switch {
		case events.testFailed[key] && stepFailure.MatchString(events.outputFor(pkg, name)):
			verdict.Red = append(verdict.Red, identity)
		case events.testPassed[key] && !events.testFailed[key]:
			verdict.Green = append(verdict.Green, identity)
		default:
			reason:=": not red at an executed step (compilation, bring-up, skipped or missing result)\n"
   evidence:=events.outputFor(pkg,name)+events.outputFor(pkg,goBuildFailureMarker)
   verdict.Invalid=append(verdict.Invalid,identity+reason+evidence)
		}
	}

	return verdict
}

// RedScenarios runs the selected suite once; ordinary test failures are verdict evidence.
func (r *GoTestRunner) RedScenarios(cfg Config, identities []string) (RedRun, error) {
	argv, err := cfg.argv()
	if err != nil {
		return RedRun{}, err
	}

	argv = appendGoScenarioFilter(argv, identities)
	argv = append(argv, "-count=1")
	stdout, stderr, runErr := r.exec(CommandDir(cfg), "red-without-target", argv)

	verdict := AssessRedRun(stdout.Bytes(), identities)
	if runErr != nil && !errors.Is(runErr, cli.ErrExit) {
		return verdict, fmt.Errorf("run without target: %w\n%s", runErr, stderr.String())
	}

	return verdict, nil
}
