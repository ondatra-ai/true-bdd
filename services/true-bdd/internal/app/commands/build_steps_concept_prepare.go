package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// ErrPatternProposal stops a concept whose proposal the engine still refuses
// after the configured attempts, before any skeleton is written.
var ErrPatternProposal = errors.New("pattern proposal refused before any skeleton")

// prepareConcepts groups, judges and proposes before writing any skeleton,
// then writes every skeleton and compiles the suite before any body.
func (e *buildStepsExecution) prepareConcepts(ctx context.Context) ([]*BuildStepsItem, error) {
	run, err := e.loadConceptRun()
	if err != nil {
		return nil, err
	}

	discovery, err := discoverConceptItems(ctx, e.deps, e.registryFile, e.architectureFile)
	if err != nil {
		return nil, err
	}

	arch, err := architecture.Load(e.architectureFile)
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

	err = e.prepareGeneratedTests(arch, discovery.scenarios)
	if err != nil {
		return nil, err
	}

	run.discovery, run.arch = discovery, arch
	e.concepts = run

	err = e.proposeConcepts(ctx, discovery)
	if err != nil {
		return nil, err
	}

	return discovery.items, e.skeletonsBeforeBodies(discovery, arch)
}

// skeletonsBeforeBodies writes every accepted pattern's skeleton and compiles the suite before any body.
func (e *buildStepsExecution) skeletonsBeforeBodies(discovery *conceptDiscovery,
	arch *architecture.Architecture,
) error {
	err := writeConceptSkeletons(discovery.items)
	if err != nil {
		return err
	}

	slog.Info("All step skeletons written", "items", len(discovery.items))

	for _, item := range discovery.items {
		err = describeConceptItem(item, arch, e.concepts.specification, discovery)
		if err != nil {
			return err
		}
	}

	err = e.compiler.CompileDefinitions(e.command)
	if err != nil {
		return err
	}

	return e.initializeItems(discovery.items)
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

func (e *buildStepsExecution) proposeConcepts(ctx context.Context, discovery *conceptDiscovery) error {
	for _, item := range discovery.items {
		subject := &template.ConceptProposalData{ID: conceptKey(item.Concept.Block, item.Concept.Name),
			Suite: item.Suite, Service: item.Service, Block: item.Concept.Block, Concept: item.Concept.Name,
			Existing: item.Concept.Existing, Suites: discovery.steps[item.Suite]}

		for _, step := range subject.Suites {
			if step.Item == subject.ID {
				subject.Steps = append(subject.Steps, step.Text)
			}
		}

		if len(subject.Steps) == 0 {
			slog.Info("Concept Item resumed", "suite", item.Suite, "concept", item.Label(),
				"definitions", len(item.Concept.Definitions))

			continue
		}

		err := e.proposeConcept(ctx, e.concepts.proposal, e.concepts.attempts, item, subject, "proposal")
		if err != nil {
			return err
		}
	}

	return nil
}

const defaultProposalAttempts = 5

func (e *buildStepsExecution) proposeConcept(ctx context.Context, prompt checklist.PromptWithContext, attempts int,
	item *BuildStepsItem, subject *template.ConceptProposalData, label string,
) error {
	for attempt := 1; ; attempt++ {
		id := fmt.Sprintf("%s %s %s %d", item.Suite, subject.ID, label, attempt)

		result, err := e.deps.Evaluator.EvaluateOne(ctx, subject, id, prompt, e.deps.TmpDir)
		if err != nil {
			return err
		}

		if len(subject.Patterns) > 0 {
			item.Concept.Patterns = subject.Patterns
			slog.Info("Pattern proposal accepted", "suite", item.Suite, "concept", item.Label(),
				"attempt", attempt, "patterns", len(subject.Patterns))

			return nil
		}

		slog.Warn("Pattern proposal refused", "suite", item.Suite, "concept", item.Label(),
			"attempt", attempt, "findings", result.Context)

		if attempt >= attempts {
			return fmt.Errorf("%w: %s %s after %d proposal(s):\n%s", ErrPatternProposal, item.Suite,
				item.Label(), attempt, strings.Join(result.Context, "\n"))
		}
	}
}

func writeConceptSkeletons(items []*BuildStepsItem) error {
	for _, item := range items {
		for _, pattern := range item.Concept.Patterns {
			definition, err := stepskeleton.WriteConcept(item.Suite, string(item.Concept.Block), item.Concept.Name,
				pattern.Pattern)
			if err != nil {
				return err
			}

			item.Concept.Definitions = append(item.Concept.Definitions, definition)
		}

		item.Findings = []string{"step not written yet: " + item.Label()}
	}

	nameConceptItems(items)

	return nil
}

func nameConceptItems(items []*BuildStepsItem) {
	counts := make(map[string]int)
	for _, item := range items {
		counts[conceptKey(item.Concept.Block, item.Concept.Name)]++
	}

	for _, item := range items {
		key := conceptKey(item.Concept.Block, item.Concept.Name)

		item.subjectID = strings.ReplaceAll(key, ":", "-")
		if counts[key] > 1 {
			item.subjectID = item.Suite + "-" + item.subjectID
		}
	}
}
