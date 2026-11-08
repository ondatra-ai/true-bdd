package testkit

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"regexp"
	"strings"
	"unicode"
)

// FixtureName returns the first capture of the first matching step, or the Scenario id if none matches.
func FixtureName(scenario bddgo.Scenario, patterns ...string) string {
	var pattern *regexp.Regexp
	if len(patterns) > 0 && patterns[0] != "" {
		pattern = regexp.MustCompile(patterns[0])
	}

	if pattern == nil {
		return scenario.ID
	}

	for _, step := range scenario.Steps {
		match := pattern.FindStringSubmatch(step.Text)
		if match != nil {
			return match[1]
		}
	}

	return scenario.ID
}

// TestName is the generated Go test function name obtained by CamelCasing the Scenario key.
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
