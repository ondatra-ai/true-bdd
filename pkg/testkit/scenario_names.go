package testkit

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"regexp"
	"strings"
	"unicode"
)

// fixtureNamePattern reads the fixture tree's name out of a scenario's
// Given step. Kept here, not just in the step definition, because the
// subtest name (what `-run` selects) must be known before any step runs.
var fixtureNamePattern = regexp.MustCompile(`the "([^"]+)" project tree`)

// FixtureName is the run-directory name for a scenario: the fixture tree
// it drives, falling back to the scenario id when the Given step names
// none (a scenario this suite refuses moments later anyway).
func FixtureName(scenario bddgo.Scenario) string {
	for _, step := range scenario.Steps {
		match := fixtureNamePattern.FindStringSubmatch(step.Text)
		if match != nil {
			return match[1]
		}
	}

	return scenario.ID
}

// TestName is the generated Go test function for a scenario, mirroring the
// engine's CamelCase of its key (ADR 0014) so a record hint names a filter
// that actually selects something.
func TestName(scenario bddgo.Scenario) string {
	var name strings.Builder

	name.WriteString("Test")

	segmentStart := true

	for _, char := range scenario.ID {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			segmentStart = true

			continue
		}

		if segmentStart {
			char = unicode.ToUpper(char)
		}

		name.WriteRune(char)

		segmentStart = false
	}

	return name.String()
}
