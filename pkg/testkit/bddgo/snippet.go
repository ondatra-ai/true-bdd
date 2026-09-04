package bddgo

import (
	"fmt"
	"regexp"
	"strings"
)

// snippetQuotedRun matches a double-quoted run inside a step, which is
// how a scenario writes the one thing that varies between two otherwise
// identical steps — a file name, a command, an exit code.
var snippetQuotedRun = regexp.MustCompile(`"[^"]*"`)

// snippetNumberRun matches a bare number, the other thing that varies.
var snippetNumberRun = regexp.MustCompile(`\b\d+\b`)

// undefinedReport renders the failure a scenario gets when one of its
// steps binds to nothing: a paste-ready step definition per missing
// step (godog-style) — also what `build tests --fix` consumes to author it.
func undefinedReport(scenario Scenario, undefined []Step) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "%s: %s\n", scenario.ID, scenario.Description)
	fmt.Fprintf(&buf, "  %d step(s) match no definition in this suite:\n", len(undefined))

	for _, step := range undefined {
		fmt.Fprintf(&buf, "    %s\n", step)
	}

	buf.WriteString("\n  Register them, e.g.:\n\n")

	for _, step := range undefined {
		fmt.Fprintf(&buf, "    suite.Step(`%s`,\n", snippetPattern(step.Text))
		fmt.Fprintf(&buf, "        func(s *state, args []string) error { return nil })\n")
	}

	return strings.TrimRight(buf.String(), "\n")
}

// snippetPattern turns a step's text into an anchored regexp with a
// capture group where the step had a quoted run or a number.
func snippetPattern(text string) string {
	pattern := regexp.QuoteMeta(text)

	// Applied to the ESCAPED text, so the quotes and digits matched here
	// are the ones QuoteMeta left alone — `"` and `\d` are not
	// metacharacters, so both runs survive escaping intact.
	pattern = snippetQuotedRun.ReplaceAllString(pattern, `"([^"]*)"`)
	pattern = snippetNumberRun.ReplaceAllString(pattern, `(\d+)`)

	return "^" + pattern + "$"
}
