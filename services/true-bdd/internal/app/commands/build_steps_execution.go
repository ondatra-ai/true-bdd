package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// ErrBuildStepsNotConverged distinguishes an unfinished authoring walk.
var ErrBuildStepsNotConverged = errors.New("step definitions still incomplete after fix walk")

type buildStepsExecution struct {
	deps             BuildStepsDeps
	registryFile     string
	architectureFile string
	command          testrunner.Config
	compiler         *testrunner.GoTestRunner
	shape            string
	active           *BuildStepsItem
	plan             *scenariogen.Plan
	items            []*BuildStepsItem
	attempts         map[*BuildStepsItem]int
	maxAttempts      int
	redChecked       bool
	green            []string
}

func runBuildStepsFix(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string) error {
	execution := &buildStepsExecution{deps: deps, registryFile: registryFile, architectureFile: architectureFile,
		compiler: testrunner.NewGoTestRunner(testrunner.NewArtifacts(deps.TmpDir))}

	return runner.Run(ctx, runner.Spec[*BuildStepsItem]{
		Name: "build steps", ChecklistName: "build-steps", Fix: true,
		LoadItems: execution.prepare, BeforeEvaluate: execution.beforeEvaluate,
		PostFix: execution.postFix, PostNotApplied: execution.postNotApplied,
		Finalize: execution.finalize, CheckConvergence: execution.checkConvergence,
		GetSubject: func(item *BuildStepsItem) (string, string) { return item.subjectID, item.Phrase.Template },
		Checks:     checks.Set[*BuildStepsItem]{"step-coverage": {Run: stepBodyVerdict}},

		Evaluator: deps.Evaluator, FixGenerator: deps.FixGenerator, FixApplier: deps.FixApplier,
		ChecklistLoader: deps.ChecklistLoader, DocResolver: deps.DocResolver, Renderer: deps.TableRenderer,
		UI: runner.NewFixLoopUI(deps.UserInputCollector), TmpDir: deps.TmpDir,
	})
}

func (e *buildStepsExecution) beforeEvaluate(_ context.Context, item *BuildStepsItem) (bool, error) {
	if e.active == item {
		return false, nil
	}

	shape, err := stepskeleton.Shape(item.Suite, item.Definition.Function)
	if err != nil {
		return false, fmt.Errorf("read the shape of %s before its apply: %w", item.Definition.Function, err)
	}

	e.active, e.shape = item, shape

	return false, nil
}

func (e *buildStepsExecution) postFix(_ context.Context, item *BuildStepsItem, _ string) (*BuildStepsItem, error) {
	e.redChecked = false
	e.attempts[item]++
	item.Findings = nil

	findings := []error{
		e.compiler.CompileDefinitions(e.command),
		VerifyGeneratedTests(e.plan, "."),
		e.preserveOtherBodies(item),
		item.Definition.Intact(item.Suite),
		item.Definition.Written(),
		stepDefinitionBinding(item),
	}
	for _, finding := range findings {
		if finding != nil {
			item.Findings = append(item.Findings, finding.Error())
		}
	}

	if e.attempts[item] > e.maxAttempts {
		failure := ErrBuildStepsNotConverged
		if len(e.green) > 0 {
			failure = ErrScenarioGreen
		}

		return item, fmt.Errorf("%w: phrase %s", failure, item.Phrase.Template)
	}

	slog.Info("Step body checked", "phrase", item.Phrase.Template, "findings", item.Findings)

	return item, nil
}

func (e *buildStepsExecution) preserveOtherBodies(item *BuildStepsItem) error {
	shape, err := stepskeleton.Shape(item.Suite, item.Definition.Function)
	if err != nil {
		return err
	}

	if shape != e.shape {
		return fmt.Errorf("%w: only the selected body, imports and State may change; "+
			"remove added package declarations and restore other bodies", stepskeleton.ErrSkeletonChanged)
	}

	return nil
}

func stepDefinitionBinding(item *BuildStepsItem) error {
	definitions, err := stepcoverage.LoadSuite(item.Suite)
	if err != nil {
		return err
	}

	bound, err := definitions.PhraseDefinitions(item.Definition.Pattern)
	if err != nil {
		return err
	}

	if len(bound) != 1 || bound[0].Pattern != item.Definition.Pattern {
		return fmt.Errorf("%w: phrase must have exactly one definition with the capturing pattern", ErrMissingStepDefinitions)
	}

	return nil
}

func (e *buildStepsExecution) postNotApplied(ctx context.Context, item *BuildStepsItem) (*BuildStepsItem, error) {
	return e.postFix(ctx, item, "")
}

func (e *buildStepsExecution) finalize(result *engine.Result[*BuildStepsItem]) error {
	if result.Reason != engine.Converged || !result.AllPassed {
		slog.Error("Step definition walk did not converge", "reason", result.Reason)

		if len(e.green) > 0 {
			return fmt.Errorf("%w: %v", ErrScenarioGreen, e.green)
		}

		return ErrBuildStepsNotConverged
	}

	if !e.redChecked {
		return ErrBuildStepsNotConverged
	}

	return nil
}

func stepBodyVerdict(item *BuildStepsItem, _ checks.Input) (checks.Verdict, error) {
	return checks.Verdict{Pass: len(item.Findings) == 0, Context: item.Findings}, nil
}
