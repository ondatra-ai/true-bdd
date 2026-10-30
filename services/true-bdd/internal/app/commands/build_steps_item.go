package commands

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// BuildStepsItem is one Catalogue phrase missing a definition in one go-test
// suite, or on a concept host one concept and suite.
type BuildStepsItem struct {
	subjectID  string
	Context    string
	Definition stepskeleton.Definition
	Findings   []string
	Suite      string
	Service    string
	Phrase     catalogue.Entry
	Concept    *BuildStepsConcept
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
	if item.Concept != nil {
		return fmt.Sprintf("%s concept %q", item.Concept.Block, item.Concept.Name)
	}

	return item.Phrase.Template
}

func (item *BuildStepsItem) functions() []string {
	if item.Concept == nil {
		return []string{item.Definition.Function}
	}

	names := make([]string, 0, len(item.Concept.Definitions))
	for _, definition := range item.Concept.Definitions {
		names = append(names, definition.Function)
	}

	return names
}

func (item *BuildStepsItem) definitions() []stepskeleton.Definition {
	if item.Concept == nil {
		return []stepskeleton.Definition{item.Definition}
	}

	return item.Concept.Definitions
}
