package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// ErrMissingStepDefinitions identifies concept Items still to write or uncovered Registry steps.
var ErrMissingStepDefinitions = errors.New("step definitions are missing")

// ErrBuildStepsChecklist identifies a checklist this deterministic command cannot answer.
var ErrBuildStepsChecklist = errors.New("build steps checklist must ask D: step-coverage without parameters")

// ErrBuildStepsFramework identifies a host whose suites build steps cannot write.
var ErrBuildStepsFramework = errors.New("build steps writes go-test suites only")

// BuildStepsDeps contains the dependencies of the step inventory and body writer.
type BuildStepsDeps struct {
	Critic             *validate.StepCritic
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

// RunBuildSteps lists the concept Items still to write, or writes their
// definitions, then checks every Registry step.
func RunBuildSteps(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string, fix bool) error {
	_, err := product.Load(productDocument(deps.DocResolver))
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	err = runConceptBuildSteps(ctx, deps, registryFile, architectureFile, fix)
	if err != nil {
		slog.Error("Concept build steps stopped", "error", err)
	}

	return err
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
