package commands

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

func (e *buildStepsExecution) fixSubject(
	ctx context.Context, item *BuildStepsItem, failed checklist.ValidationResult,
) (*BuildStepsItem, error) {
	if !strings.HasSuffix(failed.SectionPath, "/step-verification") {
		return item, nil
	}

	if prepared := e.fixItems[item]; prepared != nil {
		return prepared, nil
	}

	projected := *item
	concept := *item.Concept
	projected.Concept = &concept
	findings := e.verification[item]
	patternFindings := patternFindingLines(findings)

	if len(patternFindings) > 0 {
		err := e.repairPatterns(ctx, &projected, patternFindings)
		if err != nil {
			return item, err
		}

		concept.Definitions = projectedDefinitions(item, concept.Patterns)
	}

	concept.Definitions = selectedDefinitions(concept.Definitions, item.Concept.Definitions, findings)

	err := describeConceptItem(&projected, e.concepts.arch, e.concepts.specification, e.concepts.discovery, e.runtimeAPI)
	if err != nil {
		return item, err
	}

	if e.fixItems == nil {
		e.fixItems = make(map[*BuildStepsItem]*BuildStepsItem)
	}

	e.fixItems[item] = &projected

	return &projected, nil
}

func definitionNamed(definition stepskeleton.Definition, findings []validate.CritiqueFinding) bool {
	for _, finding := range findings {
		if finding.Definition == definition.Function || finding.Definition == definition.File+"::"+definition.Function {
			return true
		}
	}

	return false
}

func (e *buildStepsExecution) beforeApply(
	_ context.Context, item *BuildStepsItem, failed checklist.ValidationResult,
) error {
	prepared := e.fixItems[item]
	if !strings.HasSuffix(failed.SectionPath, "/step-verification") || prepared == nil {
		return e.allowBodies(item, item.functions())
	}

	if !slices.EqualFunc(item.Concept.Patterns, prepared.Concept.Patterns, samePattern) {
		item.Concept.Patterns = prepared.Concept.Patterns

		err := e.rewriteSkeletons(item)
		if err != nil {
			return err
		}
	}

	return e.allowBodies(item, prepared.functions())
}

func (e *buildStepsExecution) allowBodies(item *BuildStepsItem, functions []string) error {
	e.allowedFunctions = functions

	shape, err := stepskeleton.Shape(item.Suite, functions...)
	if err != nil {
		return err
	}

	e.shape, e.active = shape, item

	return nil
}

func projectedDefinitions(item *BuildStepsItem, patterns []catalogue.ProposedPattern) []stepskeleton.Definition {
	definitions := make([]stepskeleton.Definition, 0, len(patterns))
	for _, pattern := range patterns {
		definition := stepskeleton.ConceptLocation(item.Suite, string(item.Concept.Block), item.Concept.Name, pattern.Pattern)
		for _, existing := range item.Concept.Definitions {
			if existing.Pattern == pattern.Pattern {
				definition = existing
			}
		}

		definitions = append(definitions, definition)
	}

	return definitions
}

func patternFindingLines(findings []validate.CritiqueFinding) []string {
	lines := make([]string, 0)

	for _, finding := range findings {
		if finding.Pattern() {
			lines = append(lines, finding.String())
		}
	}

	return lines
}

func selectedDefinitions(
	definitions, previous []stepskeleton.Definition, findings []validate.CritiqueFinding,
) []stepskeleton.Definition {
	selected := make([]stepskeleton.Definition, 0)

	for _, definition := range definitions {
		if definitionNamed(definition, findings) || !slices.Contains(previous, definition) {
			selected = append(selected, definition)
		}
	}

	return selected
}

func samePattern(left, right catalogue.ProposedPattern) bool { return left.Pattern == right.Pattern }
