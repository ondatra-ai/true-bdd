package commands

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// BuildStepsItem is one concept and one go-test suite whose steps of that
// concept still need step definitions.
type BuildStepsItem struct {
	subjectID string
	Context   string
	Findings  []string
	Suite     string
	Service   string
	Concept   *BuildStepsConcept
}

// BuildStepsConcept is what a concept Item defines: the suite's steps of
// the concept, the patterns the engine accepted and their skeletons.
type BuildStepsConcept struct {
	Block       catalogue.Block
	Name        string
	Means       string
	Steps       []string
	Existing    []string
	Patterns    []catalogue.ProposedPattern
	Definitions []stepskeleton.Definition
	Resumed     bool
}

// Label names the Item in findings, logs and errors.
func (item *BuildStepsItem) Label() string {
	return fmt.Sprintf("%s concept %q", item.Concept.Block, item.Concept.Name)
}

func (item *BuildStepsItem) functions() []string {
	names := make([]string, 0, len(item.Concept.Definitions))
	for _, definition := range item.Concept.Definitions {
		names = append(names, definition.Function)
	}

	return names
}

func (item *BuildStepsItem) definitions() []stepskeleton.Definition {
	return item.Concept.Definitions
}
