package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// conceptRun is what a concept walk keeps once its Items load, so a
// verification finding against a pattern can send an Item back to its proposal.
type conceptRun struct {
	discovery     *conceptDiscovery
	arch          *architecture.Architecture
	specification conceptSpecification
	proposal      checklist.PromptWithContext
	rounds        map[string]int
}

// loadConceptRun reads the pattern item's F and Specification before grouping, refusing malformed term lists.
func (e *buildStepsExecution) loadConceptRun() (*conceptRun, error) {
	document, err := e.deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return nil, err
	}

	prompt, err := patternProposalPrompt(document)
	if err != nil {
		return nil, err
	}

	file := productDocument(e.deps.DocResolver)

	source, err := product.Load(file)
	if err != nil {
		return nil, err
	}

	definitions, mappings, err := stepSpecification(file, e.architectureFile)
	if err != nil {
		return nil, err
	}

	bindings, err := architecture.LoadConceptBindings(e.architectureFile)
	if err != nil {
		return nil, err
	}

	return &conceptRun{proposal: prompt,
		specification: conceptSpecification{source.Concepts, definitions, mappings, bindings},
		rounds:        make(map[string]int)}, nil
}

func patternProposalPrompt(document *checklist.Checklist) (checklist.PromptWithContext, error) {
	index := 0

	for _, section := range document.Sections {
		for _, prompt := range section.ValidationPrompts {
			index++

			if prompt.Check != patternBindingsID || prompt.ShouldSkip() {
				continue
			}

			err := prompt.ShapeError()
			if err != nil {
				return checklist.PromptWithContext{}, err
			}

			prompt.Question, prompt.Check = prompt.FixTemplate, ""
			prompt.With = map[string]any{"concepts": "propose"}

			return checklist.PromptWithContext{SectionID: buildStepsChecklistName, CriterionID: section.ID,
				Index: index, Prompt: prompt, DefaultModels: document.Engine, DefaultDocs: document.DefaultDocs}, nil
		}
	}

	return checklist.PromptWithContext{}, ErrBuildStepsChecklist
}
