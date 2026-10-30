package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// ErrConceptGrouping stops unlinked or ambiguous steps before skeleton creation.
var ErrConceptGrouping = errors.New("concept grouping failed before any skeleton")

// ErrConceptBinding stops Items without an unambiguous service binding.
var ErrConceptBinding = errors.New("concept binding failed before any skeleton")

// ErrPatternProposalPending marks the deliberately unfinished concept writer.
var ErrPatternProposalPending = errors.New("concept pattern proposal is not built yet; no skeletons written")

func runConceptBuildSteps(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string) error {
	scenarios, err := deps.RegistryLoader.Load(registryFile)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	arch, err := architecture.Load(architectureFile)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate concept host: %w", err)
	}

	_, err = scenariogen.BuildPlan(scenarios, arch, root, registryFile)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	answer, err := stepcoverage.Scan(scenarios, root)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	return groupAndJudgeConceptSuites(ctx, deps, unboundConceptSuites(scenarios, answer))
}

func groupAndJudgeConceptSuites(ctx context.Context, deps BuildStepsDeps, suites []*template.ScenarioCheckData) error {
	document, err := deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return err
	}

	limit := defaultGroupingBatchTexts
	if document.Config != nil && document.Config.GroupingBatchTexts != 0 {
		limit = document.Config.GroupingBatchTexts
	}

	if limit < 1 {
		return runner.RefuseStartup("build steps", errGroupingBatchSize)
	}

	prompt, err := conceptBuildPrompt(document, "group")
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	slog.Info("Loaded prompts", "command", "build steps", "items", len(suites), "prompts", 1)

	return groupAndJudgeConceptItems(ctx, deps, suites, prompt, limit)
}

func groupAndJudgeConceptItems(ctx context.Context, deps BuildStepsDeps, suites []*template.ScenarioCheckData,
	prompt checklist.PromptWithContext, limit int,
) error {
	findings := make([]string, 0)

	for _, suite := range suites {
		failed, err := groupConceptSuite(ctx, deps, suite, prompt, limit)
		if err != nil {
			return err
		}

		findings = append(findings, failed...)
	}

	if len(findings) > 0 {
		return fmt.Errorf("%w:\n%s", ErrConceptGrouping, strings.Join(findings, "\n"))
	}

	return judgeConceptSuites(ctx, deps, suites)
}

func judgeConceptSuites(ctx context.Context, deps BuildStepsDeps, suites []*template.ScenarioCheckData) error {
	bindings, err := deps.ChecklistLoader.LoadFull("scen-check")
	if err != nil {
		return err
	}

	bindingPrompt, err := conceptBuildPrompt(bindings, "bindings")
	if err != nil {
		return err
	}

	for _, suite := range suites {
		result, err := deps.Evaluator.EvaluateOne(ctx, suite, suite.ID, bindingPrompt, deps.TmpDir)
		if err != nil {
			return err
		}

		if result.Status != checklist.StatusPass {
			return fmt.Errorf("%w: %s: %s", ErrConceptBinding, suite.ID, strings.Join(result.Context, "; "))
		}

		for _, link := range suite.ConceptLinks {
			slog.Info("Concept Item", "suite", suite.ID, "service", suite.Service, "block", link.Block, "concept", link.Concept)
		}
	}

	return ErrPatternProposalPending
}

func unboundConceptSuites(scenarios []*registry.RegistryScenario,
	answer *stepcoverage.Answer,
) []*template.ScenarioCheckData {
	suites := make([]*template.ScenarioCheckData, 0)
	bySuite := make(map[string]*template.ScenarioCheckData)

	for _, scenario := range scenarios {
		id := filepath.Dir(scenario.Path)

		item, ok := bySuite[id]
		if !ok {
			item = &template.ScenarioCheckData{ID: id, Service: scenario.Service}
			bySuite[id] = item
			suites = append(suites, item)
		}

		for _, step := range answer.Gaps[scenario.ID] {
			if step.ModelRun() {
				continue
			}

			if !slices.Contains(item.Phrases, step) {
				item.Phrases = append(item.Phrases, step)
			}
		}
	}

	return suites
}

func conceptBuildPrompt(document *checklist.Checklist, kind string) (checklist.PromptWithContext, error) {
	index := 0

	for _, section := range document.Sections {
		for _, prompt := range section.ValidationPrompts {
			index++

			if prompt.With["concepts"] != kind || prompt.ShouldSkip() {
				continue
			}

			err := prompt.ShapeError()
			if err != nil {
				return checklist.PromptWithContext{}, err
			}

			stem := buildStepsChecklistName
			if kind == "bindings" {
				stem = "scen-check"
			}

			return checklist.PromptWithContext{SectionID: stem, CriterionID: section.ID, Index: index, Prompt: prompt,
				DefaultModels: document.Engine, DefaultDocs: document.DefaultDocs}, nil
		}
	}

	return checklist.PromptWithContext{}, fmt.Errorf("missing %s Prompt: %w", kind, ErrBuildStepsChecklist)
}

const defaultGroupingBatchTexts = 800

var errGroupingBatchSize = errors.New("grouping_batch_texts must be positive")

func groupConceptSuite(ctx context.Context, deps BuildStepsDeps, suite *template.ScenarioCheckData,
	prompt checklist.PromptWithContext, limit int,
) ([]string, error) {
	findings := make([]string, 0)
	for _, block := range []catalogue.Block{catalogue.BlockGiven, catalogue.BlockWhen, catalogue.BlockThen} {
		steps := make([]catalogue.Step, 0)

		for _, step := range suite.Phrases {
			if step.Block == block {
				steps = append(steps, step)
			}
		}

		for batchIndex, batch := range equalStepBatches(steps, limit) {
			item := &template.ScenarioCheckData{ID: fmt.Sprintf("%s-%s-%d", suite.ID, block, batchIndex+1),
				Service: suite.Service, Phrases: batch}
			slog.Info("Grouping batch", "suite", suite.ID, "block", block, "batch", batchIndex+1, "texts", len(batch))

			result, err := deps.Evaluator.EvaluateOne(ctx, item, item.ID, prompt, deps.TmpDir)
			if err != nil {
				return nil, err
			}

			findings = append(findings, result.Context...)

			for _, link := range item.ConceptLinks {
				if !slices.Contains(suite.ConceptLinks, link) {
					suite.ConceptLinks = append(suite.ConceptLinks, link)
				}
			}
		}
	}

	suite.ConceptLinkPassed = len(findings) == 0

	return findings, nil
}

func equalStepBatches(steps []catalogue.Step, limit int) [][]catalogue.Step {
	if len(steps) == 0 {
		return nil
	}

	count := (len(steps) + limit - 1) / limit
	batches := make([][]catalogue.Step, 0, count)
	start := 0

	for index := range count {
		size := len(steps) / count
		if index < len(steps)%count {
			size++
		}

		batches = append(batches, steps[start:start+size])
		start += size
	}

	return batches
}
