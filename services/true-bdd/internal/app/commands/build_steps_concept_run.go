package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// conceptRun is what a concept walk keeps once its Items load, so a
// critique finding against a pattern can send an Item back to its proposal.
type conceptRun struct {
	discovery     *conceptDiscovery
	arch          *architecture.Architecture
	specification conceptSpecification
	proposal      checklist.PromptWithContext
	attempts      int
	rounds        map[*BuildStepsItem]int
}

// prepareConceptRun loads the proposal Prompt, its attempts and the
// Specification every concept Item's context reads.
func (e *buildStepsExecution) prepareConceptRun(discovery *conceptDiscovery, arch *architecture.Architecture) error {
	run, err := e.loadConceptRun()
	if err != nil {
		return err
	}

	run.discovery, run.arch = discovery, arch
	e.concepts = run

	return nil
}

// loadConceptRun reads them before any turn, so a malformed term list is
// refused before grouping, as a template host's is.
func (e *buildStepsExecution) loadConceptRun() (*conceptRun, error) {
	document, err := e.deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return nil, err
	}

	prompt, err := conceptBuildPrompt(document, "propose")
	if err != nil {
		return nil, err
	}

	attempts := defaultProposalAttempts
	if document.Config != nil && document.Config.MaxApplyAttempts > 0 {
		attempts = document.Config.MaxApplyAttempts
	}

	file := productDocument(e.deps.DocResolver)

	source, err := product.LoadCatalogue(file)
	if err != nil {
		return nil, err
	}

	_, definitions, mappings, err := compileStepSpecification(source, file, e.architectureFile)
	if err != nil {
		return nil, err
	}

	bindings, err := architecture.LoadConceptBindings(e.architectureFile)
	if err != nil {
		return nil, err
	}

	return &conceptRun{proposal: prompt, attempts: attempts,
		specification: conceptSpecification{source.Concepts, definitions, mappings, bindings},
		rounds:        make(map[*BuildStepsItem]int)}, nil
}
