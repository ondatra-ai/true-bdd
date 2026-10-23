package commands

import (
	"regexp"
	"strings"
)

// normalizeRuntimeFailure sets aside test identity, source prefixes and runner
// elapsed times for comparison only; the prompt retains the original output.
func normalizeRuntimeFailure(output, testName, scenarioID string) string {
	_, name, qualified := strings.Cut(testName, "::")
	if qualified {
		output = strings.ReplaceAll(output, testName, "<test>")
	} else {
		name = testName
	}

	if name != "" {
		output = strings.ReplaceAll(output, name, "<test>")
	}

	prefixes := regexp.MustCompile(`(?m)^([ \t]*)(?:[A-Za-z]:)?[^:\r\n]*?\.[^\s:\r\n]+:\d+(?::\d+)?:[ \t]*`)
	output = prefixes.ReplaceAllString(output, "${1}")
	// bddgo prints the Registry id as the test's diagnostic heading (#180).
	if scenarioID != "" {
		identity := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(scenarioID) + `($|: )`)
		output = identity.ReplaceAllString(output, "${1}<scenario>${2}")
	}

	elapsed := regexp.MustCompile(`(?m)^([ \t]*--- (?:FAIL|PASS|SKIP): [^\n]*?) \([0-9]+(?:\.[0-9]+)?s\)$`)

	return elapsed.ReplaceAllString(output, "${1}")
}
