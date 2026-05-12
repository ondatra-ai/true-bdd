package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"bdd-cli/src/internal/app/generators/validate"
	checklistmodels "bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/infrastructure/input"
	storyinfra "bdd-cli/src/internal/infrastructure/story"
	"bdd-cli/src/internal/infrastructure/template"
	"bdd-cli/src/internal/pkg/console"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	scratchRegistryFilename = "requirements.yaml"
	scratchFilePermissions  = 0o644
)

// applyDeps bundles the four apply-flavored dependencies the walk needs
// so the public method's signature stays under the linter's parameter
// budget.
type applyDeps struct {
	parser       *storyinfra.StoryScenarioParser
	evaluator    *validate.ChecklistEvaluator
	fixGenerator *validate.FixPromptGenerator
	fixApplier   *validate.FixApplier
}

// applySetup is the resolved state needed to run the apply walk.
type applySetup struct {
	scenarios   []*template.ScenarioApplyData
	prompts     []checklistmodels.PromptWithContext
	tmpDir      string
	scratchPath string
}

// ExecuteStoryScenarioChecklist walks every acceptance criterion in the
// refined story file `<storiesDir>/<storyNumber>-*.yaml` and runs the
// named checklist against each one. Used by `us apply`.
//
// The walk operates on a scratch copy of the canonical requirements
// file: every fix-applier invocation edits `<tmpDir>/requirements.yaml`
// in place. The canonical file is replaced atomically only when every
// scenario passes every prompt (or every prompt has been driven to PASS
// via --fix). On any failure or abort, the canonical file is left
// untouched and the scratch copy is preserved in tmp for inspection.
func (c *USValidationCommand) ExecuteStoryScenarioChecklist(
	ctx context.Context,
	storyNumber string,
	requirementsFile string,
	checklistName string,
	fix bool,
	parser *storyinfra.StoryScenarioParser,
	evaluator *validate.ChecklistEvaluator,
	fixGenerator *validate.FixPromptGenerator,
	fixApplier *validate.FixApplier,
) error {
	console.Header(
		strings.ToUpper(checklistName)+" — STORY APPLY",
		separatorWidth,
	)

	deps := applyDeps{
		parser:       parser,
		evaluator:    evaluator,
		fixGenerator: fixGenerator,
		fixApplier:   fixApplier,
	}

	setup, err := c.prepareApplyWalk(storyNumber, requirementsFile, checklistName, deps)
	if err != nil {
		return err
	}

	if len(setup.prompts) == 0 {
		console.Println(
			fmt.Sprintf("No validation prompts defined for %s. Nothing to do.", checklistName),
		)

		return nil
	}

	allPassed := c.walkApplyScenarios(ctx, setup, fix, deps)

	console.Header(
		strings.ToUpper(checklistName)+" — APPLY COMPLETE",
		separatorWidth,
	)

	return c.commitApplyWalk(allPassed, setup.scratchPath, requirementsFile)
}

// prepareApplyWalk resolves the story file, copies the canonical
// registry to a scratch location, parses scenarios, and loads the
// checklist. It is the setup half of ExecuteStoryScenarioChecklist.
func (c *USValidationCommand) prepareApplyWalk(
	storyNumber, requirementsFile, checklistName string,
	deps applyDeps,
) (*applySetup, error) {
	err := c.validateStoryNumber(storyNumber)
	if err != nil {
		return nil, fmt.Errorf("invalid story number: %w", err)
	}

	tmpDir := c.runDir.GetTmpOutPath()
	scratchPath := filepath.Join(tmpDir, scratchRegistryFilename)

	err = copyFile(requirementsFile, scratchPath)
	if err != nil {
		return nil, fmt.Errorf("failed to seed scratch registry: %w", err)
	}

	scenarios, storyPath, err := deps.parser.ParseStoryScenarios(storyNumber, scratchPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse story scenarios: %w", err)
	}

	prompts, err := c.checklistLoader.Load(checklistName)
	if err != nil {
		return nil, fmt.Errorf("failed to load checklist %q: %w", checklistName, err)
	}

	slog.Info("Apply walk starting",
		"story", storyNumber,
		"story_path", storyPath,
		"scratch", scratchPath,
		"scenarios", len(scenarios),
		"prompts", len(prompts),
	)

	return &applySetup{
		scenarios:   scenarios,
		prompts:     prompts,
		tmpDir:      tmpDir,
		scratchPath: scratchPath,
	}, nil
}

// walkApplyScenarios runs the inner per-scenario × per-prompt loop and
// reports whether every scenario ended in PASS.
func (c *USValidationCommand) walkApplyScenarios(
	ctx context.Context,
	setup *applySetup,
	fix bool,
	deps applyDeps,
) bool {
	allPassed := true

	for index, scenario := range setup.scenarios {
		console.Header(
			fmt.Sprintf("AC %d/%d: %s (lineage %s)",
				index+1, len(setup.scenarios), scenario.ACID, scenario.LineageScenarioID),
			separatorWidth,
		)

		scenarioPassed, scenarioErr := c.runApplyScenario(
			ctx, scenario, setup.prompts, setup.tmpDir, fix,
			deps.evaluator, deps.fixGenerator, deps.fixApplier,
		)
		if scenarioErr != nil {
			slog.Error("Apply scenario failed",
				"ac_id", scenario.ACID,
				"error", scenarioErr,
			)
			console.Printf("Error processing %s: %v\n", scenario.ACID, scenarioErr)

			allPassed = false

			continue
		}

		if !scenarioPassed {
			allPassed = false
		}
	}

	return allPassed
}

// commitApplyWalk decides whether to atomically replace the canonical
// requirements file with the scratch copy. Called after the walk
// completes.
func (c *USValidationCommand) commitApplyWalk(
	allPassed bool,
	scratchPath, requirementsFile string,
) error {
	if !allPassed {
		console.Printf(
			"One or more scenarios did not pass. Canonical %s left unchanged. Scratch: %s\n",
			requirementsFile, scratchPath,
		)

		return nil
	}

	err := os.Rename(scratchPath, requirementsFile)
	if err != nil {
		return fmt.Errorf("failed to commit scratch registry to %s: %w", requirementsFile, err)
	}

	console.Printf("All ACs passed. %s updated from scratch.\n", requirementsFile)

	return nil
}

// runApplyScenario drives one (scenario × all prompts) cell column with
// the optional fix loop. Returns (passed, error). When passed=false in
// non-fix mode the caller continues with the next scenario but the
// final canonical-file commit is skipped.
func (c *USValidationCommand) runApplyScenario(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	prompts []checklistmodels.PromptWithContext,
	tmpDir string,
	fix bool,
	evaluator *validate.ChecklistEvaluator,
	fixGenerator *validate.FixPromptGenerator,
	fixApplier *validate.FixApplier,
) (bool, error) {
	for iteration := 1; ; iteration++ {
		shouldContinue, passed, err := c.runApplyIteration(
			ctx, scenario, prompts, tmpDir, fix, iteration,
			evaluator, fixGenerator, fixApplier,
		)
		if err != nil {
			return false, err
		}

		if !shouldContinue {
			return passed, nil
		}
	}
}

// runApplyIteration evaluates the prompts once and either confirms
// success (passed=true), reports a non-fix failure (passed=false), or
// drives a single fix turn and asks the loop to re-evaluate
// (shouldContinue=true).
func (c *USValidationCommand) runApplyIteration(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	prompts []checklistmodels.PromptWithContext,
	tmpDir string,
	fix bool,
	iteration int,
	evaluator *validate.ChecklistEvaluator,
	fixGenerator *validate.FixPromptGenerator,
	fixApplier *validate.FixApplier,
) (bool, bool, error) {
	report, err := c.evaluateApplyScenario(ctx, scenario, prompts, tmpDir, fix, evaluator)
	if err != nil {
		return false, false, err
	}

	c.tableRenderer.RenderReport(report, fix)

	if report.AllPassed() {
		console.Header("ALL CHECKS PASSED!", separatorWidth)

		return false, true, nil
	}

	failedCheck := c.getFirstFailedCheck(report)
	if failedCheck == nil {
		slog.Warn("No failed check found despite not all passed")

		return false, false, nil
	}

	c.displayFailureInfo(failedCheck)

	if !fix {
		console.BlankLine()
		console.Printf("Validation failed. Use --fix flag to enter interactive fix mode.\n")

		return false, false, nil
	}

	shouldRetry, retryErr := c.runApplyFixLoop(
		ctx, scenario, *failedCheck, tmpDir, iteration,
		fixGenerator, fixApplier,
	)
	if retryErr != nil {
		return false, false, retryErr
	}

	return shouldRetry, false, nil
}

func (c *USValidationCommand) evaluateApplyScenario(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	prompts []checklistmodels.PromptWithContext,
	tmpDir string,
	fix bool,
	evaluator *validate.ChecklistEvaluator,
) (*checklistmodels.ChecklistReport, error) {
	if fix {
		return c.evaluateApplyUntilFailure(ctx, scenario, prompts, tmpDir, evaluator)
	}

	return c.evaluateApplyAll(ctx, scenario, prompts, tmpDir, evaluator)
}

func (c *USValidationCommand) evaluateApplyUntilFailure(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	prompts []checklistmodels.PromptWithContext,
	tmpDir string,
	evaluator *validate.ChecklistEvaluator,
) (*checklistmodels.ChecklistReport, error) {
	report, err := evaluator.EvaluateUntilFailure(
		ctx, scenario, scenario.LineageScenarioID, scenario.Description, prompts, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate checklist: %w", err)
	}

	return report, nil
}

func (c *USValidationCommand) evaluateApplyAll(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	prompts []checklistmodels.PromptWithContext,
	tmpDir string,
	evaluator *validate.ChecklistEvaluator,
) (*checklistmodels.ChecklistReport, error) {
	report, err := evaluator.Evaluate(
		ctx, scenario, scenario.LineageScenarioID, scenario.Description, prompts, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate checklist: %w", err)
	}

	return report, nil
}

// runApplyFixLoop generates a fix prompt and walks the user through
// apply / refine / exit. Returning (true, nil) signals the outer
// iteration to re-evaluate after a successful apply.
func (c *USValidationCommand) runApplyFixLoop(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	iteration int,
	fixGenerator *validate.FixPromptGenerator,
	fixApplier *validate.FixApplier,
) (bool, error) {
	userAnswers := make(map[string]string)
	refinementCount := 0

	fixPrompt, answers, err := c.generateApplyFixPrompt(
		ctx, scenario, failedCheck, tmpDir, userAnswers, fixGenerator,
	)
	if err != nil {
		return false, fmt.Errorf("failed to generate fix prompt: %w", err)
	}

	userAnswers = answers

	if fixPrompt == "" {
		return false, nil
	}

	for {
		c.displayFixPrompt(fixPrompt)

		action := c.userInputCollector.AskApplyRefineOrExit()

		shouldContinue, handled, loopErr := c.handleApplyFixAction(
			ctx, action, scenario, &fixPrompt, &userAnswers, &refinementCount,
			failedCheck, tmpDir, iteration, fixGenerator, fixApplier,
		)
		if loopErr != nil {
			return false, loopErr
		}

		if handled {
			return shouldContinue, nil
		}
	}
}

// handleApplyFixAction processes a single user choice from the fix loop
// (apply / refine / exit). Returns (shouldContinue, handled, error).
// `handled=true` means the loop is done for this fix attempt;
// `handled=false` means we stay in the inner refinement loop.
//
// to mirror the existing test fix-loop signature (see runTestFixLoop /
// generateTestFixPrompt) — the engine treats failed checks as immutable
// snapshots, and copying preserves that invariant.
//

func (c *USValidationCommand) handleApplyFixAction(
	ctx context.Context,
	action input.ActionChoice,
	scenario *template.ScenarioApplyData,
	fixPrompt *string,
	userAnswers *map[string]string,
	refinementCount *int,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	iteration int,
	fixGenerator *validate.FixPromptGenerator,
	fixApplier *validate.FixApplier,
) (bool, bool, error) {
	switch action {
	case input.ActionApply:
		shouldContinue, applyErr := c.applyApplyFix(
			ctx, scenario, *fixPrompt, tmpDir, iteration, fixApplier,
		)

		return shouldContinue, true, applyErr

	case input.ActionRefine:
		c.handleApplyRefine(
			ctx, scenario, failedCheck, tmpDir, fixPrompt, userAnswers, refinementCount,
			fixGenerator,
		)

		return false, false, nil

	case input.ActionExit:
		console.Println("Exiting apply for this AC.")

		return false, true, nil
	}

	return false, true, nil
}

func (c *USValidationCommand) handleApplyRefine(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	fixPrompt *string,
	userAnswers *map[string]string,
	refinementCount *int,
	fixGenerator *validate.FixPromptGenerator,
) {
	if *refinementCount >= maxRefinementIterations {
		console.Printf(
			"\nMax refinement attempts (%d) reached.\n",
			maxRefinementIterations,
		)

		return
	}

	*refinementCount++

	newPrompt, updatedAnswers, refineErr := c.refineApplyFixPrompt(
		ctx, scenario, failedCheck, tmpDir, *userAnswers, *refinementCount,
		fixGenerator,
	)
	if refineErr != nil {
		slog.Error("Refinement failed", "error", refineErr)

		return
	}

	if newPrompt == "" {
		console.Println("\nNo feedback provided. Keeping current fix prompt.")

		return
	}

	*fixPrompt = newPrompt
	*userAnswers = updatedAnswers

	console.Printf(
		"\n(Refinement %d of %d)\n",
		*refinementCount, maxRefinementIterations,
	)
}

func (c *USValidationCommand) generateApplyFixPrompt(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	initialAnswers map[string]string,
	fixGenerator *validate.FixPromptGenerator,
) (string, map[string]string, error) {
	userAnswers := make(map[string]string, len(initialAnswers))

	for id, answer := range initialAnswers {
		userAnswers[id] = answer
	}

	for iteration := 1; iteration <= maxClarificationIterations; iteration++ {
		params := validate.GenerateParams{
			Subject:     scenario,
			SubjectID:   scenario.LineageScenarioID,
			FailedCheck: failedCheck,
			TmpDir:      tmpDir,
			UserAnswers: userAnswers,
			Iteration:   iteration,
		}

		result, err := fixGenerator.Generate(ctx, params)
		if err != nil {
			return "", userAnswers, pkgerrors.ErrFixPromptGenerationFailed(err)
		}

		if result.HasFixPrompt() {
			return result.FixPrompt, userAnswers, nil
		}

		if !result.HasQuestions() {
			return "", userAnswers, nil
		}

		answers := c.userInputCollector.AskQuestions(result.Questions)

		for id, answer := range answers {
			userAnswers[id] = answer
		}
	}

	return "", userAnswers, nil
}

func (c *USValidationCommand) refineApplyFixPrompt(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	existingAnswers map[string]string,
	refinementIteration int,
	fixGenerator *validate.FixPromptGenerator,
) (string, map[string]string, error) {
	feedback := c.userInputCollector.AskRefinementFeedback()
	if feedback == "" {
		return "", existingAnswers, nil
	}

	existingAnswers["_user_refinement"] = feedback

	params := validate.GenerateParams{
		Subject:     scenario,
		SubjectID:   scenario.LineageScenarioID,
		FailedCheck: failedCheck,
		TmpDir:      tmpDir,
		UserAnswers: existingAnswers,
		Iteration:   refinementIteration + maxClarificationIterations,
	}

	result, err := fixGenerator.Generate(ctx, params)
	if err != nil {
		return "", existingAnswers, pkgerrors.ErrFixPromptGenerationFailed(err)
	}

	if !result.HasFixPrompt() {
		return "", existingAnswers, nil
	}

	return result.FixPrompt, existingAnswers, nil
}

// applyApplyFix runs the fix-applier; the scratch registry mutation
// happens inside the Claude turn via the Edit tool, so the returned
// content is treated as informational and discarded.
func (c *USValidationCommand) applyApplyFix(
	ctx context.Context,
	scenario *template.ScenarioApplyData,
	fixPrompt string,
	tmpDir string,
	iteration int,
	fixApplier *validate.FixApplier,
) (bool, error) {
	_, err := fixApplier.Apply(
		ctx, scenario, scenario.LineageScenarioID, fixPrompt, tmpDir, iteration,
	)
	if err != nil {
		return false, fmt.Errorf("failed to apply fix: %w", err)
	}

	console.Printf("Fix applied to scratch %s. Re-running validation...\n",
		scenario.RequirementsScratchPath)

	return true, nil
}

// copyFile makes a byte-for-byte copy of src at dst, creating dst's
// parent directory if needed.
func copyFile(src, dst string) error {
	err := os.MkdirAll(filepath.Dir(dst), storyDirPermissions)
	if err != nil {
		return fmt.Errorf("failed to create scratch directory: %w", err)
	}

	srcFile, err := os.Open(filepath.Clean(src))
	if err != nil {
		return fmt.Errorf("failed to open source %s: %w", src, err)
	}

	defer func() {
		_ = srcFile.Close()
	}()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, scratchFilePermissions)
	if err != nil {
		return fmt.Errorf("failed to open destination %s: %w", dst, err)
	}

	defer func() {
		_ = dstFile.Close()
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy %s -> %s: %w", src, dst, err)
	}

	return nil
}
