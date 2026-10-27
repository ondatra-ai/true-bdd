package testrunner

import (
	"context"
	"fmt"
)

// RunSet reruns all selected members in one runner start, retaining their own verdicts.
func (r *GoTestRunner) RunSet(_ context.Context, members []*FailingTest) error {
	if len(members) == 0 {
		return nil
	}

	argv, err := setArgv(members, goTestRunFlag, goSetPattern)
	if err != nil {
		return err
	}

	stdout, stderr, runErr := r.exec(CommandDir(members[0].RunnerConfig), PhaseRerun, argv)
	if runErr != nil && stdout.Len() == 0 {
		return fmt.Errorf("go set rerun failed: %w (stderr: %s)", runErr, stderr.String())
	}

	events := parseGoTestEvents(stdout.Bytes())

	if runErr != nil && len(events.toFailingTests("", "")) == 0 {
		return runErr
	}

	events.recordSet(members)

	return nil
}

func goSetPattern(name string) (string, bool, error) {
	_, test, err := splitGoTestName(name)
	if err != nil {
		return "", false, err
	}

	return buildGoRunFilter(test), test == goBuildFailureMarker, nil
}

func (events *goEventStream) recordSet(members []*FailingTest) {
	for _, member := range members {
		pkg, test, _ := splitGoTestName(member.TestName)

		passed := events.testPassed[pkg+"\x00"+test] && !events.hasFailureFor(pkg, test)
		if test == goBuildFailureMarker {
			passed = events.buildPassed(pkg)
		}

		output := events.outputFor(pkg, test)
		if output == "" && events.failedBuild[pkg] != "" {
			output = events.outputFor(pkg, goBuildFailureMarker)
		}

		if !passed && !events.hasFailureFor(pkg, test) {
			output = member.TestName + " did not run: no passing result of its own\n" + output
		}

		recordMember(member, passed, output)
	}
}
