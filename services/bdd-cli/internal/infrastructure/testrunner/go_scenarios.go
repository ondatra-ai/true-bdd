package testrunner

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrScenarioNotRun prevents an empty or skipped selection from passing.
var ErrScenarioNotRun = errors.New("declared suite did not pass the selected Scenario")

func appendGoScenarioFilter(argv, tests []string) []string {
	if len(tests) == 0 {
		return argv
	}

	escaped := make([]string, 0, len(tests))
	for _, test := range tests {
		parts := strings.Split(test, goTestNameSeparator)
		name := parts[len(parts)-1]

		escaped = append(escaped, regexp.QuoteMeta(name))
	}

	return append(argv, goTestRunFlag, "^("+strings.Join(escaped, "|")+")$")
}

func (s *goEventStream) requireTests(tests []string) error {
	for _, test := range tests {
		pkg, name, err := splitGoTestName(test)
		if err != nil {
			return err
		}

		if !s.testPassed[pkg+"\x00"+name] {
			return fmt.Errorf("%w: %s", ErrScenarioNotRun, test)
		}
	}

	return nil
}
