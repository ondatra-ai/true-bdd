package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// PlannedTests is the set of top-level tests a `go test` invocation
// intends to run, matching -run/-skip as an UNANCHORED regexp on the
// first path element, exactly as `go test` does (see TestPlannedTestsRunIsUnanchored).
func PlannedTests(names []string, runPattern, skipPattern string) ([]string, error) {
	run, err := splitFilter(runPattern)
	if err != nil {
		return nil, fmt.Errorf("-run: %w", err)
	}

	skip, err := splitFilter(skipPattern)
	if err != nil {
		return nil, fmt.Errorf("-skip: %w", err)
	}

	planned := make([]string, 0, len(names))

	for _, name := range names {
		// An absent -run selects everything; an absent -skip drops nothing.
		if len(run) > 0 && !run[0].MatchString(name) {
			continue
		}

		if len(skip) > 0 && skip[0].MatchString(name) {
			continue
		}

		planned = append(planned, name)
	}

	return planned, nil
}

// splitFilter compiles one filter flag into per-level patterns. An empty
// pattern yields no elements, meaning "no filter at this level".
func splitFilter(pattern string) ([]*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}

	parts := strings.Split(pattern, "/")
	elements := make([]*regexp.Regexp, 0, len(parts))

	for _, part := range parts {
		expr, err := regexp.Compile(part)
		if err != nil {
			return nil, fmt.Errorf("compile %q: %w", part, err)
		}

		elements = append(elements, expr)
	}

	return elements, nil
}
