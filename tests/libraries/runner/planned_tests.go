package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// PlannedTests is the set of top-level tests a `go test` invocation
// intends to run: everything the binary declares, minus what its -run
// and -skip patterns filter out.
//
// It exists so a report can say "6 / 0 / 19" while the suite is still
// working. The alternative — counting the tests that already left a
// directory — makes the denominator grow as the run proceeds, so it can
// only ever say how much of what has finished is green, never how much
// of the run remains.
//
// The testing package keeps its matcher private, so this reimplements
// it rather than reading it: split the pattern on "/" and match the
// first element as an UNANCHORED regexp against the test's name (which
// is why -run TestFoo also runs TestFooBar). Only the first element,
// because there are no subtests left to address — one scenario is one
// top-level test now, and a filter naming a deeper level is `testing`'s
// business rather than this function's.
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
