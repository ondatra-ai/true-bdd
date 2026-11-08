package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// prepareConcepts groups and judges the Items; the checklist owns patterns and skeletons.
func (e *buildStepsExecution) prepareConcepts(ctx context.Context) ([]*BuildStepsItem, error) {
	run, err := e.loadConceptRun()
	if err != nil {
		return nil, err
	}

	arch, err := architecture.Load(e.architectureFile)
	if err != nil {
		return nil, err
	}

	err = e.prepareRuntimeAPI(ctx, arch)
	if err != nil {
		return nil, err
	}

	discovery, err := discoverConceptItems(ctx, e.deps, e.registryFile, e.architectureFile)
	if err != nil {
		return nil, err
	}

	err = conceptInterfaces(discovery.items, arch)
	if err != nil {
		return nil, err
	}

	err = e.configure(ctx, arch, discovery.scenarios)
	if err != nil {
		return nil, err
	}

	run.discovery, run.arch = discovery, arch
	e.concepts = run

	nameConceptItems(discovery.items)

	for _, item := range discovery.items {
		err = describeConceptItem(item, arch, run.specification, discovery, e.runtimeAPI)
		if err != nil {
			return nil, err
		}

		item.Findings = []string{"step not written yet: " + item.Label()}
	}


	return discovery.items, nil
}

func conceptInterfaces(items []*BuildStepsItem, arch *architecture.Architecture) error {
	for _, item := range items {
		for _, service := range arch.Services {
			if service.Name != item.Service {
				continue
			}

			if finding := service.PublicInterface.Finding(); finding != "" {
				return fmt.Errorf("%w: %s: %s", ErrStepSpecification, item.Service, finding)
			}
		}
	}

	return nil
}

func (e *buildStepsExecution) proposeConcept(ctx context.Context, prompt checklist.PromptWithContext,
	item *BuildStepsItem, subject *template.ConceptProposalData, label string,
) error {
	id := fmt.Sprintf("%s %s %s", item.Suite, subject.ID, label)

	_, err := e.deps.Evaluator.EvaluateOne(ctx, subject, id, prompt, e.deps.TmpDir)
	if err != nil {
		return err
	}

	item.Concept.Patterns = subject.Patterns

	return nil
}

func nameConceptItems(items []*BuildStepsItem) {
	counts := make(map[string]int)
	for _, item := range items {
		counts[catalogue.ConceptKey(item.Concept.Block, item.Concept.Name)]++
	}

	for _, item := range items {
		key := catalogue.ConceptKey(item.Concept.Block, item.Concept.Name)

		item.subjectID = strings.ReplaceAll(key, ":", "-")
		if counts[key] > 1 {
			item.subjectID = item.Suite + "-" + item.subjectID
		}
	}
}
