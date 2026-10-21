package catalogue

import (
	"fmt"
	"strings"
)

// Review is what the catalogue says of a run of steps: how many matched
// one template in their own block, how many a model runs, and one
// finding per step that did neither.
type Review struct {
	Matched  int
	Exempt   int
	Findings []string
}

// Review matches every step a model does not run against the whole
// catalogue: exactly one template must match, declared for the step's block.
func (c *Catalogue) Review(steps []Step) Review {
	review := Review{Findings: make([]string, 0)}

	for _, step := range steps {
		if step.ModelRun() {
			review.Exempt++

			continue
		}

		finding := c.finding(step)
		if finding == "" {
			review.Matched++

			continue
		}

		review.Findings = append(review.Findings, finding)
	}

	return review
}

// finding is why one step is not phrased by exactly one template of its
// block, or "" when it is.
func (c *Catalogue) finding(step Step) string {
	matches := c.Match(step.Text)
	subject := fmt.Sprintf("%s step %s", step.Block, quote(step.Text))

	switch len(matches) {
	case 0:
		return subject + " matches no template"
	case 1:
		if matches[0].Block == step.Block {
			return ""
		}

		return fmt.Sprintf("%s matches %s, declared for %s", subject, quote(matches[0].Template), matches[0].Block)
	}

	templates := make([]string, 0, len(matches))
	for _, match := range matches {
		templates = append(templates, quote(match.Template))
	}

	return fmt.Sprintf("%s matches %d templates: %s", subject, len(matches), strings.Join(templates, "; "))
}

// quote wraps a step or a template in double quotes, verbatim: most carry
// quotes of their own, and escaping them would bury the text a reader needs.
func quote(text string) string {
	return `"` + text + `"`
}
