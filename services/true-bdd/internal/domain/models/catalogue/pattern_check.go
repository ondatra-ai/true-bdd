package catalogue

import (
	"fmt"
	"regexp"
	"strings"
)

// CheckPatterns holds one Item's proposal to every step of its suite in every
// block, as the harness binds: a pattern binds only the Item's own steps, each
// of them exactly one pattern, and every pattern some step.
func CheckPatterns(item string, patterns []ProposedPattern, steps []SuiteStep) []string {
	findings := make([]string, 0)
	compiled := make([]*regexp.Regexp, 0, len(patterns))

	for _, proposed := range patterns {
		pattern, err := regexp.Compile(proposed.Pattern)
		if err != nil {
			findings = append(findings, fmt.Sprintf("pattern %s does not compile: %v", proposed.Pattern, err))

			continue
		}

		if pattern.NumSubexp() != len(proposed.Captures) {
			findings = append(findings, fmt.Sprintf("pattern %s has %d capture groups but names %d capture(s)",
				proposed.Pattern, pattern.NumSubexp(), len(proposed.Captures)))
		}

		findings = append(findings, foreignBindings(item, pattern, steps)...)
		compiled = append(compiled, pattern)
	}

	if len(compiled) < len(patterns) {
		return findings
	}

	return append(findings, ownBindings(item, compiled, steps)...)
}

// foreignBindings refuses a pattern binding no step, or a step its Item
// does not hold: one another definition binds or another Item groups.
func foreignBindings(item string, pattern *regexp.Regexp, steps []SuiteStep) []string {
	findings := make([]string, 0)
	used := false

	for _, step := range steps {
		if !pattern.MatchString(step.Text) {
			continue
		}

		used = true

		switch {
		case step.Item == item:
		case step.Item != "":
			findings = append(findings, fmt.Sprintf("pattern %s also binds %s step %q, which is grouped into %s",
				pattern, step.Block, step.Text, step.Item))
		default:
			findings = append(findings, fmt.Sprintf("pattern %s also binds %s step %q, which the definition %s binds",
				pattern, step.Block, step.Text, step.Bound))
		}
	}

	if !used {
		findings = append(findings, fmt.Sprintf("pattern %s binds no step of the suite", pattern))
	}

	return findings
}

// ownBindings requires each step the Item holds to bind exactly one pattern.
func ownBindings(item string, patterns []*regexp.Regexp, steps []SuiteStep) []string {
	findings := make([]string, 0)

	for _, step := range steps {
		if step.Item != item {
			continue
		}

		bound := make([]string, 0, 1)

		for _, pattern := range patterns {
			if pattern.MatchString(step.Text) {
				bound = append(bound, pattern.String())
			}
		}

		switch len(bound) {
		case 0:
			findings = append(findings, fmt.Sprintf("no pattern binds %s step %q", step.Block, step.Text))
		case 1:
		default:
			findings = append(findings, fmt.Sprintf("%s step %q binds %d patterns: %s",
				step.Block, step.Text, len(bound), strings.Join(bound, " | ")))
		}
	}

	return findings
}

// PatternCaptures lists the arguments a pattern captures from a step, in order.
func PatternCaptures(pattern, text string) []string {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	match := compiled.FindStringSubmatch(text)
	if len(match) == 0 {
		return nil
	}

	return match[1:]
}
