package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// PlannedFixtures is the set of fixtures a `go test` invocation intends
// to run: everything discovered, minus what its -run and -skip patterns
// filter out. rootTest is the name of the parent test the fixtures are
// subtests of.
//
// It exists so a report can say "6 / 0 / 19" while the suite is still
// working. The alternative — counting the fixtures that already left a
// directory — makes the denominator grow as the run proceeds, so it can
// only ever say how much of what has finished is green, never how much
// of the run remains.
//
// The testing package keeps its matcher private, so this reimplements
// it rather than reading it: split the pattern on "/", match each
// element as an UNANCHORED regexp against the corresponding element of
// the test's name path (which is why -run TestFoo also runs TestFooBar).
// Deliberately not reimplemented: the `\/` escape, and the #NN suffix
// testing appends to duplicate subtest names. Neither can arise here —
// fixture names are directory names, and directory names are unique.
func PlannedFixtures(rootTest string, names []string, runPattern, skipPattern string) ([]string, error) {
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
		if len(run) > 0 && !filterMatches(run, rootTest, name) {
			continue
		}

		if len(skip) > 0 && filterMatches(skip, rootTest, name) {
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

// filterMatches reports whether one fixture's subtest path satisfies a
// split filter. A filter naming only the parent matches every fixture
// beneath it; one whose first element does not match the parent matches
// nothing at all.
func filterMatches(elements []*regexp.Regexp, rootTest, fixture string) bool {
	if !elements[0].MatchString(rootTest) {
		return false
	}

	if len(elements) == 1 {
		return true
	}

	return elements[1].MatchString(fixture)
}
