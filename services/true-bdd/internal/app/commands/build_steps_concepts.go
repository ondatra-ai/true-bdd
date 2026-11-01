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
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// ErrConceptGrouping stops unlinked or ambiguous steps before skeleton creation.
var ErrConceptGrouping = errors.New("concept grouping failed before any skeleton")

// ErrConceptBinding stops Items without an unambiguous service binding.
var ErrConceptBinding = errors.New("concept binding failed before any skeleton")

func runConceptBuildSteps(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string,
	fix bool,
) error {
	if fix {
		return runBuildStepsFix(ctx, deps, registryFile, architectureFile)
	}

	discovery, err := discoverConceptItems(ctx, deps, registryFile, architectureFile)
	if err != nil {
		return err
	}

	return listConceptItems(deps, discovery)
}

// conceptDiscovery is what a concept walk starts from: the Registry, its
// unbound steps, and one Item per concept and suite, grouped or resumed.
type conceptDiscovery struct {
	scenarios []*registry.RegistryScenario
	answer    *stepcoverage.Answer
	items     []*BuildStepsItem
	steps     map[string][]catalogue.SuiteStep
}

func discoverConceptItems(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string,
) (*conceptDiscovery, error) {
	scenarios, answer, err := scanConceptHost(deps, registryFile, architectureFile)
	if err != nil {
		return nil, err
	}

	suites, err := conceptSuites(deps, scenarios, answer)
	if err != nil {
		return nil, err
	}

	asked, err := loadConceptJudgements(deps)
	if err != nil {
		return nil, err
	}

	groups, err := groupConceptSuites(ctx, deps, suites, asked)
	if err != nil {
		return nil, err
	}

	discovery := &conceptDiscovery{scenarios: scenarios, answer: answer}

	err = discovery.collect(suites, groups)
	if err != nil {
		return nil, err
	}

	err = judgeConceptBindings(ctx, deps, suites, discovery.items, asked.bindings)
	if err != nil {
		return nil, err
	}

	return discovery, nil
}

// scanConceptHost reads the Registry, refuses one build tests could not render, and scans its unbound steps.
func scanConceptHost(deps BuildStepsDeps, registryFile, architectureFile string,
) ([]*registry.RegistryScenario, *stepcoverage.Answer, error) {
	scenarios, err := deps.RegistryLoader.Load(registryFile)
	if err != nil {
		return nil, nil, runner.RefuseStartup("build steps", err)
	}

	arch, err := architecture.Load(architectureFile)
	if err != nil {
		return nil, nil, runner.RefuseStartup("build steps", err)
	}

	root, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("locate concept host: %w", err)
	}

	_, err = scenariogen.BuildPlan(scenarios, arch, root, registryFile)
	if err != nil {
		return nil, nil, runner.RefuseStartup("build steps", err)
	}

	answer, err := stepcoverage.Scan(scenarios, root)
	if err != nil {
		return nil, nil, runner.RefuseStartup("build steps", err)
	}

	return scenarios, answer, nil
}

// conceptJudgements are the grouping Prompt, its batch size and scen check's bindings Prompt, read before any turn.
type conceptJudgements struct {
	group    checklist.PromptWithContext
	limit    int
	bindings checklist.PromptWithContext
}

func loadConceptJudgements(deps BuildStepsDeps) (conceptJudgements, error) {
	document, err := deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return conceptJudgements{}, runner.RefuseStartup("build steps", err)
	}

	limit := defaultGroupingBatchTexts
	if document.Config != nil && document.Config.GroupingBatchTexts != 0 {
		limit = document.Config.GroupingBatchTexts
	}

	if limit < 1 {
		return conceptJudgements{}, runner.RefuseStartup("build steps", errGroupingBatchSize)
	}

	group, err := conceptBuildPrompt(document, checklist.JudgementGroup)
	if err != nil {
		return conceptJudgements{}, runner.RefuseStartup("build steps", err)
	}

	scenCheck, err := deps.ChecklistLoader.LoadFull(scenCheckChecklistName)
	if err != nil {
		return conceptJudgements{}, runner.RefuseStartup("build steps", err)
	}

	bindings, err := conceptBuildPrompt(scenCheck, checklist.JudgementBindings)
	if err != nil {
		return conceptJudgements{}, runner.RefuseStartup("build steps", err)
	}

	return conceptJudgements{group: group, limit: limit, bindings: bindings}, nil
}

// conceptSuites lists the suites whose unbound steps grouping reads. A host
// declaring no concepts has none: its unbound steps stay Step coverage's findings.
func conceptSuites(deps BuildStepsDeps, scenarios []*registry.RegistryScenario,
	answer *stepcoverage.Answer,
) ([]*template.ScenarioCheckData, error) {
	source, err := product.Load(productDocument(deps.DocResolver))
	if err != nil {
		return nil, runner.RefuseStartup("build steps", err)
	}

	if !source.ConceptsDeclared {
		return make([]*template.ScenarioCheckData, 0), nil
	}

	return unboundConceptSuites(scenarios, answer), nil
}

func groupConceptSuites(ctx context.Context, deps BuildStepsDeps, suites []*template.ScenarioCheckData,
	asked conceptJudgements,
) (map[string]map[catalogue.Step]string, error) {
	slog.Info("Loaded prompts", "command", "build steps", "items", len(suites), "prompts", 1)

	findings := make([]string, 0)
	groups := make(map[string]map[catalogue.Step]string)

	for _, suite := range suites {
		failed, grouped, err := groupConceptSuite(ctx, deps, suite, asked.group, asked.limit)
		if err != nil {
			return nil, err
		}

		findings = append(findings, failed...)
		groups[suite.ID] = grouped
	}

	if len(findings) > 0 {
		return nil, fmt.Errorf("%w:\n%s", ErrConceptGrouping, strings.Join(findings, "\n"))
	}

	return groups, nil
}

// judgeConceptBindings judges each binding as scen check does (ADR-0037): one turn per binding, over the steps
// of every Item that reaches its concept for the service.
func judgeConceptBindings(ctx context.Context, deps BuildStepsDeps, suites []*template.ScenarioCheckData,
	items []*BuildStepsItem, bindingPrompt checklist.PromptWithContext,
) error {
	deps.Evaluator.StartWalk()

	for _, item := range items {
		deps.Evaluator.ShareSteps(item.Suite+" "+item.Label(), item.Service,
			template.ConceptLink{Concept: item.Concept.Name, Block: item.Concept.Block}, item.Concept.Steps)
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

	return nil
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

			if !slices.Contains(item.CheckedSteps, step) {
				item.CheckedSteps = append(item.CheckedSteps, step)
			}
		}
	}

	return suites
}

func conceptBuildPrompt(document *checklist.Checklist, kind checklist.ConceptJudgement,
) (checklist.PromptWithContext, error) {
	index := 0

	for _, section := range document.Sections {
		for _, prompt := range section.ValidationPrompts {
			index++

			if prompt.Judgement() != kind || prompt.ShouldSkip() {
				continue
			}

			err := prompt.ShapeError()
			if err != nil {
				return checklist.PromptWithContext{}, err
			}

			stem := buildStepsChecklistName
			if kind == checklist.JudgementBindings {
				stem = scenCheckChecklistName
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
) ([]string, map[catalogue.Step]string, error) {
	findings := make([]string, 0)
	grouped := make(map[catalogue.Step]string)

	for _, block := range []catalogue.Block{catalogue.BlockGiven, catalogue.BlockWhen, catalogue.BlockThen} {
		steps := make([]catalogue.Step, 0)

		for _, step := range suite.CheckedSteps {
			if step.Block == block {
				steps = append(steps, step)
			}
		}

		for batchIndex, batch := range equalStepBatches(steps, limit) {
			item := &template.ScenarioCheckData{ID: fmt.Sprintf("%s-%s-%d", suite.ID, block, batchIndex+1),
				Service: suite.Service, CheckedSteps: batch}
			slog.Info("Grouping batch", "suite", suite.ID, "block", block, "batch", batchIndex+1, "texts", len(batch))

			result, err := deps.Evaluator.EvaluateOne(ctx, item, item.ID, prompt, deps.TmpDir)
			if err != nil {
				return nil, nil, err
			}

			findings = append(findings, result.Context...)

			if len(item.ConceptLinks) == len(batch) {
				for index, step := range batch {
					grouped[step] = item.ConceptLinks[index].Concept
				}
			}

			for _, link := range item.ConceptLinks {
				if !slices.Contains(suite.ConceptLinks, link) {
					suite.ConceptLinks = append(suite.ConceptLinks, link)
				}
			}
		}
	}

	suite.ConceptLinkPassed = len(findings) == 0

	return findings, grouped, nil
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

// listConceptItems answers a bare run: every concept Item still to write,
// and the Step coverage of the whole Registry.
func listConceptItems(deps BuildStepsDeps, discovery *conceptDiscovery) error {
	prompts, err := buildStepsPrompts(deps.ChecklistLoader, deps.DocResolver)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	for _, item := range discovery.items {
		slog.Error("Missing step definition", "suite", item.Suite, "concept", item.Label(),
			"steps", len(item.Concept.Steps), "resumed", item.Concept.Resumed)
	}

	findings := stepCoverageFindings(discovery.answer)
	for _, finding := range findings {
		slog.Error("Step coverage failed", "finding", finding)
	}

	// Grouping logged this run's one "Loaded prompts" record already.
	err = recordStepCoverage(deps.TmpDir, prompts, findings, len(discovery.items), false)
	if err != nil {
		return err
	}

	if len(discovery.items) > 0 || len(findings) > 0 {
		return fmt.Errorf("%w: %d Item(s), %d uncovered step(s)", ErrMissingStepDefinitions,
			len(discovery.items), len(findings))
	}

	slog.Info("Step coverage passed", "examined", len(discovery.answer.Examined))
	slog.Info("BUILD STEPS COMPLETE")

	return nil
}
