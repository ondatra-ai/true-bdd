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
	critiqueTier     string
	redChecked       bool
	green            []string
	concepts         *conceptRun
}

func runBuildStepsFix(ctx context.Context, deps BuildStepsDeps, registryFile, architectureFile string) error {
	execution := &buildStepsExecution{deps: deps, registryFile: registryFile, architectureFile: architectureFile,
		compiler: testrunner.NewGoTestRunner(testrunner.NewArtifacts(deps.TmpDir))}

	err := execution.prepareCritique()
	if err != nil {
		return runner.RefuseStartup("build steps", err)
	}

	return runner.Run(ctx, runner.Spec[*BuildStepsItem]{
		Name: "build steps", ChecklistName: buildStepsChecklistName, Fix: true, ConfineClaudeWriter: true,
		OmitConceptJudgements: true, PromptsLogged: true, LoadItems: execution.prepareConcepts,
		BeforeEvaluate: execution.beforeEvaluate,
		PostFix:        execution.postFix, PostNotApplied: execution.postNotApplied,
		Finalize: execution.finalize, CheckConvergence: execution.checkConvergence,
		GetSubject: func(item *BuildStepsItem) (string, string) { return item.subjectID, item.Label() },
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

	shape, err := stepskeleton.Shape(item.Suite, item.functions()...)
	if err != nil {
		return false, fmt.Errorf("read the shape of %s before its apply: %w", item.Label(), err)
	}

	e.active, e.shape = item, shape

	return false, nil
}

func (e *buildStepsExecution) postFix(ctx context.Context, item *BuildStepsItem, _ string) (*BuildStepsItem, error) {
	e.redChecked = false
	e.attempts[item]++
	item.Findings = nil

	findings := append(append([]error{
		e.compiler.CompileDefinitions(e.command),
		VerifyGeneratedTests(e.plan, "."),
		e.preserveOtherBodies(item),
	}, definitionFindings(item)...), conceptDefinitionBinding(item), conceptRecorded(item))

	for _, finding := range findings {
		if finding != nil {
			item.Findings = append(item.Findings, finding.Error())
		}
	}

	// The critique reads only a body that compiles, binds and is written:
	// one that fails those checks already returns to the Fix loop.
	if len(item.Findings) == 0 {
		findings, err := e.critique(ctx, item)
		if err != nil {
			return item, err
		}

		err = e.applyCritique(ctx, item, findings)
		if err != nil {
			return item, err
		}
	}

	if e.attempts[item] > e.maxAttempts {
		failure := ErrBuildStepsNotConverged
		if len(e.green) > 0 {
			failure = ErrScenarioGreen
		}

		return item, fmt.Errorf("%w: %s", failure, item.Label())
	}

	slog.Info("Step body checked", "concept", item.Label(), "findings", item.Findings)

	return item, nil
}

// definitionFindings holds each of the Item's skeletons intact and written.
func definitionFindings(item *BuildStepsItem) []error {
	findings := make([]error, 0, len(item.definitions()))
	for _, definition := range item.definitions() {
		findings = append(findings, definition.Intact(item.Suite), definition.Written())
	}

	return findings
}

func (e *buildStepsExecution) preserveOtherBodies(item *BuildStepsItem) error {
	shape, err := stepskeleton.Shape(item.Suite, item.functions()...)
	if err != nil {
		return err
	}

	if shape != e.shape {
		return fmt.Errorf("%w: only the selected body, imports and State may change; "+
			"remove added package declarations and restore other bodies", stepskeleton.ErrSkeletonChanged)
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
