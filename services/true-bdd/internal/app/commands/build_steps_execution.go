package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// ErrBuildStepsNotConverged distinguishes an unfinished authoring walk.
var ErrBuildStepsNotConverged = errors.New("step definitions still incomplete after fix walk")

type buildStepsExecution struct {
	scenarioRunApplies int
	verification       map[*BuildStepsItem][]validate.CritiqueFinding
	fixItems           map[*BuildStepsItem]*BuildStepsItem
	allowedFunctions   []string
	runtimeAPI         map[string]string
	readRoots          []string
	deps               BuildStepsDeps
	registryFile       string
	architectureFile   string
	compiler           *testrunner.GoTestRunner
	shape              string
	active             *BuildStepsItem
	concepts           *conceptRun
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
		BeforeEvaluate: execution.beforeEvaluate, Asks: asksBuildSteps,
		GenerateFix: execution.generateStepFix, ApplyFix: execution.applyStepFix,
		AnswerPrompt: execution.answerVerification, FixSubject: execution.fixSubject, BeforeApply: execution.beforeApply,
		PostFix: execution.postFix, PostNotApplied: execution.postNotApplied,
		Finalize: execution.finalize, CheckConvergence: execution.checkConvergence,
		GetSubject: func(item *BuildStepsItem) (string, string) { return item.subjectID, item.Label() },
		Checks: checks.Set[*BuildStepsItem]{
			scenarioRunID:  {Run: scenarioRunVerdict},
			stepCoverageID: {Run: stepBodyVerdict}, patternBindingsID: {Run: execution.patternBindings},
		},

		Evaluator: deps.Evaluator, FixGenerator: deps.FixGenerator, FixApplier: deps.FixApplier,
		ChecklistLoader: deps.ChecklistLoader, DocResolver: deps.DocResolver, Renderer: deps.TableRenderer,
		UI: runner.NewFixLoopUI(deps.UserInputCollector), TmpDir: deps.TmpDir,
	})
}

func (e *buildStepsExecution) beforeEvaluate(_ context.Context, item *BuildStepsItem) (bool, error) {
	if item.Concept == nil || e.active == item {
		return false, nil
	}

	shape, err := stepskeleton.Shape(item.Suite, item.functions()...)
	if err != nil {
		return false, fmt.Errorf("read the shape of %s before its apply: %w", item.Label(), err)
	}

	e.active, e.shape, e.allowedFunctions = item, shape, item.functions()

	return false, nil
}

func (e *buildStepsExecution) postFix(_ context.Context, item *BuildStepsItem, _ string) (*BuildStepsItem, error) {
	item.Findings = nil

	findings := append(append([]error{
		e.compiler.CompileDefinitions([]string{item.Suite}),
		e.preserveOtherBodies(item),
	}, definitionFindings(item)...), conceptDefinitionBinding(item), conceptRecorded(item))

	for _, finding := range findings {
		if finding != nil {
			item.Findings = append(item.Findings, finding.Error())
		}
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
	shape, err := stepskeleton.Shape(item.Suite, e.allowedFunctions...)
	if err != nil {
		return err
	}

	if shape != e.shape {
		return fmt.Errorf("%w: only the selected bodies, imports, State and helpers/ may change; "+
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

		return ErrBuildStepsNotConverged
	}

	return nil
}

func stepBodyVerdict(item *BuildStepsItem, _ checks.Input) (checks.Verdict, error) {
	return checks.Verdict{Pass: len(item.Findings) == 0, Context: item.Findings}, nil
}
