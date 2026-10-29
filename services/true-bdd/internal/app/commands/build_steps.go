package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// ErrBuildStepsFixUnavailable refuses the not-yet-implemented body writer.
var ErrBuildStepsFixUnavailable = errors.New("build steps --fix is not implemented yet")

// ErrMissingStepDefinitions identifies missing phrase definitions or uncovered Registry steps.
var ErrMissingStepDefinitions = errors.New("step definitions are missing")

// ErrBuildStepsChecklist identifies a checklist this deterministic command cannot answer.
var ErrBuildStepsChecklist = errors.New("build steps checklist must ask D: step-coverage without parameters")

// BuildStepsDeps contains the dependencies of the deterministic step inventory.
type BuildStepsDeps struct {
	RegistryLoader  *registry.RegistryLoader
	ChecklistLoader *checklist.ChecklistLoader
	DocResolver     *docs.Resolver
	TmpDir          string
}

// RunBuildSteps lists missing phrase definitions and checks every Registry step in-process.
func RunBuildSteps(_ context.Context, deps BuildStepsDeps, registryFile, architectureFile string, fix bool) error {
	if fix {
		return runner.RefuseStartup("build steps", ErrBuildStepsFixUnavailable)
	}

	prompts, err := buildStepsPrompts(deps.ChecklistLoader, deps.DocResolver)
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	items, answer, err := discoverBuildSteps(deps, registryFile, architectureFile)
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
	deps BuildStepsDeps, registryFile, architectureFile string,
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

	answer, err := stepcoverage.Scan(scenarios, root)
	if err != nil {
		return nil, nil, err
	}

	compiled, _ := catalogue.Compile(source.Entries, source.Roles)
	items, err := missingPhrases(scenarios, source.Entries, compiled)

	return items, answer, err
}

func missingPhrases(
	scenarios []*registry.RegistryScenario, phrases []catalogue.Entry, compiled *catalogue.Catalogue,
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
			defined, err := suite.Defines(compiled.Pattern(phrase.Template))
			if err != nil {
				return nil, err
			}

			if !defined {
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
