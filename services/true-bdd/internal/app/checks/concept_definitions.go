package checks

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// reviewConceptDefinitions holds a concept host's Scenario to its suite's
// step definitions: no step binds two, and the one a step binds records a
// declared concept of that step's block. An unbound step is build steps' gap.
func reviewConceptDefinitions(item *template.ScenarioCheckData, source product.Document, dir string,
) (Verdict, error) {
	suite, err := stepcoverage.LoadSuite(dir)
	if err != nil {
		return Verdict{}, fmt.Errorf("read the step definitions in %s: %w", dir, err)
	}

	recorded, err := stepskeleton.Concepts(dir)
	if err != nil {
		return Verdict{}, fmt.Errorf("read the concept records in %s: %w", dir, err)
	}

	findings, unbound := reviewConceptSteps(item.CheckedSteps, suite, recorded, source.Concepts)
	if len(findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(item.ID, findings)}, nil
	}

	line := item.ID + ": each step binds one step definition, which records a declared concept of its block, in " + dir

	switch len(unbound) {
	case 0:
	case 1:
		line += "; 1 step unbound yet"
	default:
		line += fmt.Sprintf("; %d steps unbound yet", len(unbound))
	}

	return Verdict{Pass: true, Context: append([]string{line}, prefixed(item.ID, unbound)...)}, nil
}

func reviewConceptSteps(steps []catalogue.Step, suite *stepcoverage.Suite,
	recorded []stepskeleton.ConceptDefinition, concepts []product.Concept,
) ([]string, []string) {
	findings, unbound := make([]string, 0), make([]string, 0)

	for _, step := range steps {
		if step.ModelRun() {
			continue
		}

		switch bound := suite.Bind(step.Text); len(bound) {
		case 0:
			unbound = append(unbound, fmt.Sprintf("%s step %q binds to no step definition yet", step.Block, step.Text))
		case 1:
			if finding := conceptRecordFinding(step, bound[0], recorded, concepts); finding != "" {
				findings = append(findings, finding)
			}
		default:
			findings = append(findings, fmt.Sprintf("%s step %q binds to %d step definitions: %s",
				step.Block, step.Text, len(bound), definitionPatterns(bound)))
		}
	}

	return findings, unbound
}

// conceptRecordFinding names a bound definition recording no declared concept of the step's block.
func conceptRecordFinding(step catalogue.Step, definition catalogue.StepDefinition,
	recorded []stepskeleton.ConceptDefinition, concepts []product.Concept,
) string {
	where := fmt.Sprintf("%s step %q binds to the step definition %s (%s:%d)", step.Block, step.Text,
		definition.Pattern, definition.File, definition.Line)

	index := slices.IndexFunc(recorded, func(record stepskeleton.ConceptDefinition) bool {
		return record.Pattern == definition.Pattern
	})
	if index < 0 {
		return where + ", which records no concept"
	}

	record := recorded[index]
	if record.Block == string(step.Block) && slices.ContainsFunc(concepts, func(concept product.Concept) bool {
		return concept.Block == step.Block && concept.Name == record.Concept
	}) {
		return ""
	}

	return fmt.Sprintf("%s, which records %s concept %q, which is not a declared %s concept", where,
		record.Block, record.Concept, step.Block)
}

func definitionPatterns(bound []catalogue.StepDefinition) string {
	patterns := make([]string, 0, len(bound))
	for _, definition := range bound {
		patterns = append(patterns, fmt.Sprintf("%s (%s:%d)", definition.Pattern, definition.File, definition.Line))
	}

	return strings.Join(patterns, "; ")
}
