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
		escaped = append(escaped, regexp.QuoteMeta(test))
	}

	return append(argv, goTestRunFlag, "^("+strings.Join(escaped, "|")+")$")
}

func (s *goEventStream) requireTests(tests []string) error {
	for _, test := range tests {
		if !s.testPassed[test] {
			return fmt.Errorf("%w: %s", ErrScenarioNotRun, test)
		}
	}

	return nil
}
