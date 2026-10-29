package main

import (
	"regexp"
	"slices"
)

// A cassette opts into this rule; legacy hashes retain their byte contract.
var goDurationFrame = regexp.MustCompile(`(?m)^([ \t]*--- (?:FAIL|PASS|SKIP): [^\n]+ \()[0-9]+\.[0-9]+s(\))$`)

func hasGoDuration(argv []string, stdin []byte) bool {
	return goDurationFrame.Match(stdin) || slices.ContainsFunc(argv, goDurationFrame.MatchString)
}

func canonicalGoRequest(argv []string, stdin []byte, enabled bool) ([]string, []byte) {
	if !enabled {
		return argv, stdin
	}

	result := make([]string, len(argv))
	for index, arg := range argv {
		result[index] = goDurationFrame.ReplaceAllString(arg, "${1}0.00s${2}")
	}

	return result, goDurationFrame.ReplaceAll(stdin, []byte("${1}0.00s${2}"))
}
