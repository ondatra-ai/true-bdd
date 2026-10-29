package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// ErrMissingStepDefinitions identifies missing phrase definitions or uncovered Registry steps.
var ErrMissingStepDefinitions = errors.New("step definitions are missing")

// ErrBuildStepsChecklist identifies a checklist this deterministic command cannot answer.
var ErrBuildStepsChecklist = errors.New("build steps checklist must ask D: step-coverage without parameters")

// ErrBuildStepsFramework identifies a host whose suites build steps cannot write.
var ErrBuildStepsFramework = errors.New("build steps writes go-test suites only")

// BuildStepsDeps contains the dependencies of the step inventory and body writer.
type BuildStepsDeps struct {
	Evaluator          *validate.ChecklistEvaluator
	FixGenerator       *validate.FixPromptGenerator
	FixApplier         *validate.FixApplier
	UserInputCollector *input.UserInputCollector
	TableRenderer      *runner.TableRenderer
	RegistryLoader     *registry.RegistryLoader
	ChecklistLoader    *checklist.ChecklistLoader
	DocResolver        *docs.Resolver
	TmpDir             string
}

// RunBuildSteps lists missing phrases or writes their bodies, then checks every Registry step.
func RunBuildSteps(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string, fix bool) error {
	if fix {
		return runBuildStepsFix(ctx, deps, registryFile, architectureFile)
	}

	prompts, err := buildStepsPrompts(deps.ChecklistLoader, deps.DocResolver)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	items, answer, err := discoverBuildSteps(deps, registryFile, architectureFile, true)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	for _, item := range items {
		slog.Error("Missing step definition", "suite", item.Suite, "phrase", item.Phrase.Template)
	}

	findings := stepCoverageFindings(answer)
	for _, finding := range findings {
		slog.Error("Step coverage failed", "finding", finding)
	}

	err = recordStepCoverage(deps.TmpDir, prompts, findings, len(items))
	if err != nil {
		return err
	}

	if len(items) > 0 || len(findings) > 0 {
		return fmt.Errorf("%w: %d Item(s), %d uncovered step(s)", ErrMissingStepDefinitions, len(items), len(findings))
	}

	slog.Info("Step coverage passed", "examined", len(answer.Examined))
	slog.Info("BUILD STEPS COMPLETE")

	return nil
}

func discoverBuildSteps(
	deps BuildStepsDeps, registryFile, architectureFile string, scanCoverage bool,
) ([]*BuildStepsItem, *stepcoverage.Answer, error) {
	scenarios, err := deps.RegistryLoader.Load(registryFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load Registry: %w", err)
	}

	arch, err := architecture.Load(architectureFile)
	if err != nil {
		return nil, nil, err
	}

	root, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve working directory: %w", err)
	}

	_, err = scenariogen.BuildPlan(scenarios, arch, root, registryFile)
	if err != nil {
		return nil, nil, err
	}

	prep := &buildTestsPrep{productFile: productDocument(deps.DocResolver)}

	source, err := prep.holdToCatalogue(scenarios)
	if err != nil {
		return nil, nil, err
	}

	answer := &stepcoverage.Answer{}
	if scanCoverage {
		answer, err = stepcoverage.Scan(scenarios, root)
		if err != nil {
			return nil, nil, err
		}
	}

	compiled, _ := catalogue.Compile(source.Entries, source.Roles)
	items, err := missingPhrases(scenarios, source.Entries, compiled, arch.Phrases)

	return items, answer, err
}

func missingPhrases(
	scenarios []*registry.RegistryScenario, phrases []catalogue.Entry,
	compiled *catalogue.Catalogue, mappings []architecture.PhraseMapping,
) ([]*BuildStepsItem, error) {
	items := make([]*BuildStepsItem, 0)
	seen := make(map[string]bool)

	for _, scenario := range scenarios {
		dir := stepcoverage.StepsDir(scenario.Path)
		if seen[dir] {
			continue
		}

		seen[dir] = true

		suite, err := stepcoverage.LoadSuite(dir)
		if err != nil {
			return nil, err
		}

		for _, phrase := range phrases {
			if !phraseBelongs(scenarios, dir, scenario.Service, phrase.Template, compiled, mappings) {
				continue
			}

			defined, err := suite.Defines(compiled.Pattern(phrase.Template))
			if err != nil {
				return nil, err
			}

			location := stepskeleton.Location(dir, phrase.Template, compiled.CapturingPattern(phrase.Template))
			if !defined || location.Pending() {
				items = append(items, &BuildStepsItem{Suite: dir, Service: scenario.Service, Phrase: phrase})
			}
		}
	}

	return items, nil
}

func stepCoverageFindings(answer *stepcoverage.Answer) []string {
	findings := make([]string, 0)

	for id, gaps := range answer.Gaps {
		for _, gap := range gaps {
			findings = append(findings, fmt.Sprintf("%s: unbound %s step: %s", id, gap.Block, gap.Text))
		}
	}
	// A stable order also makes repeated runs report exactly the same failures.
	slices.Sort(findings)

	return findings
}

func phraseBelongs(scenarios []*registry.RegistryScenario, dir, service, template string,
	compiled *catalogue.Catalogue, mappings []architecture.PhraseMapping) bool {
	for _, mapping := range mappings {
		if mapping.Template == template && mapping.Service == service {
			return true
		}
	}

	for _, scenario := range scenarios {
		if stepcoverage.StepsDir(scenario.Path) != dir {
			continue
		}

		for _, step := range scenario.Phrases() {
			for _, phrase := range compiled.Match(step.Text) {
				if phrase.Template == template {
					return true
				}
			}
		}
	}

	return false
}
