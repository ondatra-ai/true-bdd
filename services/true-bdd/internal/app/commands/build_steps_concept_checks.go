package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// conceptDefinitionBinding requires every step of the concept Item to bind
// exactly one definition, one of the Item's own.
func conceptDefinitionBinding(item *BuildStepsItem) error {
	definitions, err := stepcoverage.LoadSuite(item.Suite)
	if err != nil {
		return err
	}

	patterns := make([]string, 0, len(item.Concept.Definitions))
	for _, definition := range item.Concept.Definitions {
		patterns = append(patterns, definition.Pattern)
	}

	findings := make([]string, 0)

	for _, step := range item.Concept.Steps {
		bound := definitions.Bind(step)
		if len(bound) != 1 || !slices.Contains(patterns, bound[0].Pattern) {
			findings = append(findings, fmt.Sprintf("step %q binds %d definition(s)", step, len(bound)))
		}
	}

	if len(findings) > 0 {
		return fmt.Errorf("%w: every step of %s must bind exactly one of its definitions: %s",
			ErrMissingStepDefinitions, item.Label(), strings.Join(findings, "; "))
	}

	return nil
}

// conceptRecorded keeps each skeleton's record of its concept, which a resumed walk reads.
func conceptRecorded(item *BuildStepsItem) error {
	recorded, err := stepskeleton.Concepts(item.Suite)
	if err != nil {
		return err
	}

	for _, definition := range item.Concept.Definitions {
		found := false

		for _, candidate := range recorded {
			found = found || (candidate.Function == definition.Function &&
				candidate.Block == string(item.Concept.Block) && candidate.Concept == item.Concept.Name)
		}

		if !found {
			return fmt.Errorf("%w: keep the //truebdd:concept record of %s", stepskeleton.ErrSkeletonChanged,
				definition.Function)
		}
	}

	return nil
}

func conceptItemUsed(item *BuildStepsItem, scenario *registry.RegistryScenario) bool {
	for _, step := range scenario.Phrases() {
		if step.Block == item.Concept.Block && slices.Contains(item.Concept.Steps, step.Text) {
			return true
		}
	}

	return false
}

func definitionLabels(item *BuildStepsItem) string {
	labels := make([]string, 0, len(item.definitions()))
	for _, definition := range item.definitions() {
		labels = append(labels, definition.File+"::"+definition.Function)
	}

	return strings.Join(labels, ", ")
}
