package testrunner

import (
	"strings"
	"time"
)

// setArgv keeps the declared command and joins the members' native selectors.
func setArgv(members []*FailingTest, selector string, pattern func(string) (string, bool, error)) ([]string, error) {
	argv, err := commandArgv(members[0])
	if err != nil {
		return nil, err
	}

	patterns := make([]string, 0, len(members))
	unfiltered := false

	for _, member := range members {
		value, synthetic, err := pattern(member.TestName)
		if err != nil {
			return nil, err
		}

		unfiltered = unfiltered || synthetic

		patterns = append(patterns, value)
	}

	if unfiltered {
		return argv, nil
	}

	return append(argv, selector, strings.Join(patterns, "|")), nil
}

func recordMember(member *FailingTest, passed bool, output string) {
	member.LastRunPassed = passed
	member.FailureOutput = output

	member.LastRunAt = time.Now()
}

func recordOutcomes(members []*FailingTest, outcomes map[string]dtoOutcome) {
	for _, member := range members {
		result := outcomes[member.TestName]
		if !result.ran {
			result.output = member.TestName + " did not run: no passing result of its own\n" + result.output
		}

		recordMember(member, result.passed, result.output)
	}
}
