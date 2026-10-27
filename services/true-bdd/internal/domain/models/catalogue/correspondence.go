package catalogue

import (
	"fmt"
	"slices"
	"strings"
)

// Correspondence is what a suite's step definitions say of one Item's
// steps: one finding per template, definition or step breaking the one to
// one relation, and one line per step no definition binds yet.
type Correspondence struct {
	Findings []string
	Unbound  []string
}

// relation is which definitions each template binds to, and which
// templates reach each definition, over every step of one suite.
type relation struct {
	definitions map[string][]StepDefinition
	templates   map[StepDefinition][]string
}

// Correspond holds the templates an Item's steps match to the definitions
// binding them. The relation spans suite, the Item's own steps among
// them, so a template two Items bind apart fails both.
func (c *Catalogue) Correspond(item, suite []Step, bind Binder, dir string) Correspondence {
	rel := c.relate(suite, bind)
	review := Correspondence{Findings: make([]string, 0), Unbound: make([]string, 0)}
	reported := make(map[string]bool)

	for _, step := range item {
		template, matched := c.phrase(step)
		if !matched {
			continue
		}

		bound := bind(step.Text)

		if definitions := rel.definitions[template]; len(definitions) > 1 {
			review.report(reported, fmt.Sprintf("template %s binds to %d step definitions in %s: %s",
				quote(template), len(definitions), dir, labels(definitions)))
		}

		switch len(bound) {
		case 0:
			review.Unbound = append(review.Unbound, fmt.Sprintf(
				"%s step %s binds to no step definition yet", step.Block, quote(step.Text)))
		case 1:
			if templates := rel.templates[bound[0]]; len(templates) > 1 {
				review.report(reported, fmt.Sprintf("step definition %s is reached by %d templates: %s",
					bound[0].label(), len(templates), quoteAll(templates)))
			}
		default:
			slices.SortFunc(bound, compareDefinitions)
			review.report(reported, fmt.Sprintf("%s step %s binds to %d step definitions: %s",
				step.Block, quote(step.Text), len(bound), labels(bound)))
		}
	}

	return review
}

// report adds a finding once, however many of the Item's steps lead to it.
func (r *Correspondence) report(reported map[string]bool, finding string) {
	if reported[finding] {
		return
	}

	reported[finding] = true

	r.Findings = append(r.Findings, finding)
}

// relate builds the relation from every step of the suite that matches
// one template and binds to one definition, each side in a stable order.
func (c *Catalogue) relate(suite []Step, bind Binder) relation {
	rel := relation{
		definitions: make(map[string][]StepDefinition),
		templates:   make(map[StepDefinition][]string),
	}

	for _, step := range suite {
		template, matched := c.phrase(step)
		if !matched {
			continue
		}

		bound := bind(step.Text)
		if len(bound) != 1 {
			continue
		}

		if !slices.Contains(rel.definitions[template], bound[0]) {
			rel.definitions[template] = append(rel.definitions[template], bound[0])
		}

		if !slices.Contains(rel.templates[bound[0]], template) {
			rel.templates[bound[0]] = append(rel.templates[bound[0]], template)
		}
	}

	for template := range rel.definitions {
		slices.SortFunc(rel.definitions[template], compareDefinitions)
	}

	for definition := range rel.templates {
		slices.SortFunc(rel.templates[definition], c.compareTemplates)
	}

	return rel
}

// phrase is the template of the one entry a step a model does not run
// matches; a step matching none, or several, is the step-phrase check's.
func (c *Catalogue) phrase(step Step) (string, bool) {
	if step.ModelRun() {
		return "", false
	}

	matches := c.Match(step.Text)
	if len(matches) != 1 {
		return "", false
	}

	return matches[0].Template, true
}

// compareTemplates orders templates as the catalogue lists them.
func (c *Catalogue) compareTemplates(left, right string) int {
	position := func(template string) int {
		return slices.IndexFunc(c.entries, func(candidate compiledEntry) bool {
			return candidate.entry.Template == template
		})
	}

	return position(left) - position(right)
}

// quoteAll quotes each template, as a finding lists them.
func quoteAll(templates []string) string {
	quoted := make([]string, 0, len(templates))
	for _, template := range templates {
		quoted = append(quoted, quote(template))
	}

	return strings.Join(quoted, "; ")
}
