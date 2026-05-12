package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/app/generators/validate"
	checklistmodels "bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/domain/models/story"
	"bdd-cli/src/internal/infrastructure/checklist"
	"bdd-cli/src/internal/infrastructure/epic"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/infrastructure/input"
	storyinfra "bdd-cli/src/internal/infrastructure/story"
	"bdd-cli/src/internal/pkg/console"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	maxClarificationIterations = 5
	maxRefinementIterations    = 3
	separatorWidth             = 80
	storyFilePermissions       = 0o644
	storyDirPermissions        = 0o755
)

// CommandConfig describes a single `us` subcommand: which checklist file it
// drives and how it sources its story entity.
type CommandConfig struct {
	// CommandName is the subcommand name (e.g. "us create"). Used for error
	// messages.
	CommandName string

	// ChecklistName is the checklist filename stem (e.g. "us-create"). The
	// loader appends ".yaml" and prepends the configured checklists dir.
	ChecklistName string

	// LoadFromEpic controls how the story is loaded:
	//   true  -> extract from epic.
	//   false -> load from docs/stories/<id>-*.yaml.
	LoadFromEpic bool
}

// USValidationCommand validates user stories against a per-command checklist.
type USValidationCommand struct {
	epicLoader         *epic.EpicLoader
	storyLoader        *storyinfra.StoryLoader
	checklistLoader    *checklist.ChecklistLoader
	checklistEvaluator *validate.ChecklistEvaluator
	fixPromptGenerator *validate.FixPromptGenerator
	fixApplier         *validate.FixApplier
	userInputCollector *input.UserInputCollector
	tableRenderer      *TableRenderer
	runDir             *fs.RunDirectory
	storiesDir         string
}

// NewUSValidationCommand creates a new validation command.
func NewUSValidationCommand(
	epicLoader *epic.EpicLoader,
	storyLoader *storyinfra.StoryLoader,
	checklistLoader *checklist.ChecklistLoader,
	evaluator *validate.ChecklistEvaluator,
	fixPromptGen *validate.FixPromptGenerator,
	fixApplier *validate.FixApplier,
	inputCollector *input.UserInputCollector,
	renderer *TableRenderer,
	runDir *fs.RunDirectory,
	storiesDir string,
) *USValidationCommand {
	return &USValidationCommand{
		epicLoader:         epicLoader,
		storyLoader:        storyLoader,
		checklistLoader:    checklistLoader,
		checklistEvaluator: evaluator,
		fixPromptGenerator: fixPromptGen,
		fixApplier:         fixApplier,
		userInputCollector: inputCollector,
		tableRenderer:      renderer,
		runDir:             runDir,
		storiesDir:         storiesDir,
	}
}

// Execute runs validation for a story against the given command's checklist.
func (c *USValidationCommand) Execute(
	ctx context.Context,
	storyNumber string,
	fix bool,
	config CommandConfig,
) error {
	valCtx, err := c.initializeValidation(storyNumber, config)
	if err != nil {
		return err
	}

	return c.runValidationLoop(ctx, valCtx, fix)
}

// validationContext holds the context for a validation run.
type validationContext struct {
	versionMgr  *fs.StoryVersionManager
	prompts     []checklistmodels.PromptWithContext
	tmpDir      string
	iteration   int
	cmdConfig   CommandConfig
	storyNumber string
}

func (c *USValidationCommand) initializeValidation(
	storyNumber string,
	config CommandConfig,
) (*validationContext, error) {
	err := c.validateStoryNumber(storyNumber)
	if err != nil {
		return nil, fmt.Errorf("invalid story number: %w", err)
	}

	slog.Info("Starting validation",
		"story", storyNumber,
		"command", config.CommandName,
	)

	console.Header(
		fmt.Sprintf("%s — Story %s", strings.ToUpper(config.CommandName), storyNumber),
		separatorWidth,
	)

	var originalStory *story.Story

	if config.LoadFromEpic {
		originalStory, err = c.loadFromEpic(storyNumber)
	} else {
		originalStory, err = c.loadStoryFromFile(storyNumber)
	}

	if err != nil {
		return nil, err
	}

	slog.Info("Story loaded", "id", originalStory.ID, "title", originalStory.Title)

	tmpDir := c.runDir.GetTmpOutPath()
	versionMgr := fs.NewStoryVersionManager(c.runDir, storyNumber)

	err = versionMgr.SaveInitialVersion(originalStory)
	if err != nil {
		return nil, fmt.Errorf("failed to save initial story version: %w", err)
	}

	prompts, err := c.checklistLoader.Load(config.ChecklistName)
	if err != nil {
		return nil, fmt.Errorf("failed to load checklist: %w", err)
	}

	slog.Info("Loaded prompts", "command", config.CommandName, "count", len(prompts))

	return &validationContext{
		versionMgr:  versionMgr,
		prompts:     prompts,
		tmpDir:      tmpDir,
		iteration:   0,
		cmdConfig:   config,
		storyNumber: storyNumber,
	}, nil
}

func (c *USValidationCommand) loadFromEpic(storyNumber string) (*story.Story, error) {
	console.Header("LOADING STORY FROM EPIC", separatorWidth)

	originalStory, err := c.epicLoader.LoadStoryFromEpic(storyNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to load story: %w", pkgerrors.ErrLoadStoryFromEpicFailed(err))
	}

	c.displayStory(originalStory, "STORY FROM EPIC")

	return originalStory, nil
}

func (c *USValidationCommand) loadStoryFromFile(storyNumber string) (*story.Story, error) {
	doc, err := c.storyLoader.Load(storyNumber)
	if err != nil {
		return nil, fmt.Errorf(
			"story file not found — run `bdd-cli us create %s` first: %w",
			storyNumber, err,
		)
	}

	return &doc.Story, nil
}

func (c *USValidationCommand) runValidationLoop(
	ctx context.Context,
	valCtx *validationContext,
	fix bool,
) error {
	// Empty-checklist short-circuit: if there are no prompts, the command is
	// a stub (e.g. us implement). Mark as passed and write the story file so
	// downstream tooling sees consistent state.
	if len(valCtx.prompts) == 0 {
		console.Header("NO CHECKS DEFINED", separatorWidth)
		console.Println("This command has no validation prompts yet. Marking as passed.")
		c.handleAllPassed(valCtx)

		return nil
	}

	for {
		valCtx.iteration++

		shouldContinue, err := c.runSingleIteration(ctx, valCtx, fix)
		if err != nil {
			return err
		}

		if !shouldContinue {
			return nil
		}
	}
}

func (c *USValidationCommand) runSingleIteration(
	ctx context.Context,
	valCtx *validationContext,
	fix bool,
) (bool, error) {
	currentStory, err := valCtx.versionMgr.LoadLatest()
	if err != nil {
		return false, fmt.Errorf("failed to load story version: %w", err)
	}

	report, err := c.evaluateStory(ctx, currentStory, valCtx, fix)
	if err != nil {
		return false, err
	}

	c.tableRenderer.RenderReport(report, fix)

	if report.AllPassed() {
		c.handleAllPassed(valCtx)

		return false, nil
	}

	failedCheck := c.getFirstFailedCheck(report)
	if failedCheck == nil {
		slog.Warn("No failed check found despite not all passed")

		return false, nil
	}

	c.displayFailureInfo(failedCheck)

	if !fix {
		console.BlankLine()
		console.Printf("Validation failed. Use --fix flag to enter interactive fix mode.\n")

		return false, nil
	}

	return c.runFixPromptLoop(ctx, valCtx, currentStory, *failedCheck)
}

func (c *USValidationCommand) evaluateStory(
	ctx context.Context,
	currentStory *story.Story,
	valCtx *validationContext,
	fix bool,
) (*checklistmodels.ChecklistReport, error) {
	var (
		report *checklistmodels.ChecklistReport
		err    error
	)

	if fix {
		report, err = c.checklistEvaluator.EvaluateUntilFailure(
			ctx, currentStory, currentStory.ID, currentStory.Title,
			valCtx.prompts, valCtx.tmpDir)
	} else {
		report, err = c.checklistEvaluator.Evaluate(
			ctx, currentStory, currentStory.ID, currentStory.Title,
			valCtx.prompts, valCtx.tmpDir)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to evaluate checklist: %w", err)
	}

	return report, nil
}

func (c *USValidationCommand) runFixPromptLoop(
	ctx context.Context,
	valCtx *validationContext,
	currentStory *story.Story,
	failedCheck checklistmodels.ValidationResult,
) (bool, error) {
	userAnswers := make(map[string]string)
	refinementCount := 0

	fixPrompt, answers, err := c.generateFixPromptWithAnswers(
		ctx, currentStory, failedCheck, valCtx.tmpDir, userAnswers,
	)
	if err != nil {
		return false, fmt.Errorf("failed to generate fix prompt: %w", err)
	}

	userAnswers = answers

	if fixPrompt == "" {
		slog.Warn("No fix prompt generated")

		return false, nil
	}

	for {
		c.displayFixPrompt(fixPrompt)

		action := c.userInputCollector.AskApplyRefineOrExit()

		switch action {
		case input.ActionApply:
			return c.applyFix(ctx, valCtx, currentStory, fixPrompt)

		case input.ActionRefine:
			if refinementCount >= maxRefinementIterations {
				console.Printf(
					"\nMax refinement attempts (%d) reached. Please apply or exit.\n",
					maxRefinementIterations,
				)

				continue
			}

			refinementCount++

			newPrompt, updatedAnswers, refineErr := c.refineFixPrompt(
				ctx, currentStory, failedCheck, valCtx.tmpDir,
				userAnswers, refinementCount,
			)
			if refineErr != nil {
				return false, pkgerrors.ErrFixPromptRefinementFailed(refineErr)
			}

			if newPrompt == "" {
				console.Println("\nNo feedback provided. Keeping current fix prompt.")

				continue
			}

			fixPrompt = newPrompt
			userAnswers = updatedAnswers

			console.Printf(
				"\n(Refinement %d of %d)\n",
				refinementCount, maxRefinementIterations,
			)

		case input.ActionExit:
			console.Printf(
				"\nExiting. Latest version saved at: %s\n",
				valCtx.versionMgr.GetLatestPath(),
			)

			return false, nil
		}
	}
}

func (c *USValidationCommand) refineFixPrompt(
	ctx context.Context,
	currentStory *story.Story,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	existingAnswers map[string]string,
	refinementIteration int,
) (string, map[string]string, error) {
	feedback := c.userInputCollector.AskRefinementFeedback()
	if feedback == "" {
		return "", existingAnswers, nil
	}

	existingAnswers["_user_refinement"] = feedback

	params := validate.GenerateParams{
		Subject:     currentStory,
		SubjectID:   currentStory.ID,
		FailedCheck: failedCheck,
		TmpDir:      tmpDir,
		UserAnswers: existingAnswers,
		Iteration:   refinementIteration + maxClarificationIterations,
	}

	result, err := c.fixPromptGenerator.Generate(ctx, params)
	if err != nil {
		return "", existingAnswers, pkgerrors.ErrFixPromptGenerationFailed(err)
	}

	if !result.HasFixPrompt() {
		return "", existingAnswers, nil
	}

	return result.FixPrompt, existingAnswers, nil
}

func (c *USValidationCommand) generateFixPromptWithAnswers(
	ctx context.Context,
	storyData *story.Story,
	failedCheck checklistmodels.ValidationResult,
	tmpDir string,
	initialAnswers map[string]string,
) (string, map[string]string, error) {
	userAnswers := make(map[string]string)

	for id, answer := range initialAnswers {
		userAnswers[id] = answer
	}

	for iteration := 1; iteration <= maxClarificationIterations; iteration++ {
		params := validate.GenerateParams{
			Subject:     storyData,
			SubjectID:   storyData.ID,
			FailedCheck: failedCheck,
			TmpDir:      tmpDir,
			UserAnswers: userAnswers,
			Iteration:   iteration,
		}

		result, err := c.fixPromptGenerator.Generate(ctx, params)
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

func (c *USValidationCommand) applyFix(
	ctx context.Context,
	valCtx *validationContext,
	currentStory *story.Story,
	fixPrompt string,
) (bool, error) {
	content, err := c.fixApplier.Apply(
		ctx, currentStory, currentStory.ID, fixPrompt, valCtx.tmpDir, valCtx.iteration,
	)
	if err != nil {
		return false, fmt.Errorf("failed to apply fix: %w", err)
	}

	var updatedACs []story.AcceptanceCriterion

	err = yaml.Unmarshal([]byte(content), &updatedACs)
	if err != nil {
		return false, fmt.Errorf("failed to parse updated acceptance criteria: %w", err)
	}

	updatedStory := *currentStory
	updatedStory.AcceptanceCriteria = updatedACs

	_, err = valCtx.versionMgr.SaveNextVersion(&updatedStory)
	if err != nil {
		return false, pkgerrors.ErrSaveStoryVersionFailed(err)
	}

	console.Printf(
		"\nFix applied. Saved as version %d.\n",
		valCtx.versionMgr.GetCurrentVersion(),
	)
	console.Println("Re-running validation...")

	return true, nil
}

func (c *USValidationCommand) handleAllPassed(valCtx *validationContext) {
	console.Header("ALL CHECKS PASSED!", separatorWidth)
	console.Printf("Latest version: %s\n", valCtx.versionMgr.GetLatestPath())

	c.displayFinalStory(valCtx.versionMgr)

	latestStory, err := valCtx.versionMgr.LoadLatest()
	if err != nil {
		slog.Warn("Could not load latest story for writing", "error", err)

		return
	}

	var storyPath string

	if valCtx.cmdConfig.LoadFromEpic {
		storyPath, err = c.writeNewStoryFile(latestStory)
	} else {
		storyPath, err = c.updateStoryFile(valCtx.storyNumber, latestStory)
	}

	if err != nil {
		slog.Warn("Could not write story file", "error", err)
		console.Printf("Warning: Could not write story file: %v\n", err)

		return
	}

	console.Printf("Story saved to: %s\n", storyPath)
}

func (c *USValidationCommand) writeNewStoryFile(storyData *story.Story) (string, error) {
	slug := slugify(storyData.Title)
	filename := fmt.Sprintf("%s-%s.yaml", storyData.ID, slug)
	filePath := filepath.Join(c.storiesDir, filename)

	err := os.MkdirAll(c.storiesDir, storyDirPermissions)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	wrapper := struct {
		Story story.Story `yaml:"story"`
	}{Story: *storyData}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	err = os.WriteFile(filePath, data, storyFilePermissions)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	slog.Info("Story file created", "path", filePath)

	return filePath, nil
}

func (c *USValidationCommand) updateStoryFile(
	storyNumber string,
	updatedStory *story.Story,
) (string, error) {
	pattern := filepath.Join(c.storiesDir, storyNumber+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	if len(matches) == 0 {
		return c.writeNewStoryFile(updatedStory)
	}

	filePath := matches[0]

	wrapper := struct {
		Story story.Story `yaml:"story"`
	}{Story: *updatedStory}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	err = os.WriteFile(filePath, data, storyFilePermissions)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	slog.Info("Story file updated", "path", filePath)

	return filePath, nil
}

func (c *USValidationCommand) displayStory(storyData *story.Story, header string) {
	console.BlankLine()
	console.Header(header, separatorWidth)

	yamlBytes, err := yaml.Marshal(storyData)
	if err != nil {
		slog.Warn("Could not marshal story to YAML", "error", err)

		return
	}

	console.Println(string(yamlBytes))
	console.Separator("=", separatorWidth)
}

func (c *USValidationCommand) displayFinalStory(versionMgr *fs.StoryVersionManager) {
	storyData, err := versionMgr.LoadLatest()
	if err != nil {
		slog.Warn("Could not load final story for display", "error", err)

		return
	}

	console.BlankLine()
	console.Header("FINAL STORY VERSION", separatorWidth)

	yamlBytes, err := yaml.Marshal(storyData)
	if err != nil {
		slog.Warn("Could not marshal story to YAML", "error", err)

		return
	}

	console.Println(string(yamlBytes))
	console.Separator("=", separatorWidth)
}

func (c *USValidationCommand) getFirstFailedCheck(
	report *checklistmodels.ChecklistReport,
) *checklistmodels.ValidationResult {
	for _, result := range report.Results {
		if result.Status == checklistmodels.StatusFail {
			return &result
		}
	}

	return nil
}

func (c *USValidationCommand) displayFailureInfo(
	failedCheck *checklistmodels.ValidationResult,
) {
	console.BlankLine()
	console.Separator("=", separatorWidth)
	console.Printf("CHECK FAILED: %s\n", failedCheck.SectionPath)
	console.Separator("=", separatorWidth)
	console.Printf("Question: %s\n", failedCheck.Question)

	if failedCheck.Rationale != "" {
		console.Printf("Rationale: %s\n", failedCheck.Rationale)
	}

	if len(failedCheck.Context) > 0 {
		console.Println("Context:")

		for _, line := range failedCheck.Context {
			console.Printf("  - %s\n", line)
		}
	}
}

func (c *USValidationCommand) displayFixPrompt(fixPrompt string) {
	console.BlankLine()
	console.Header("FIX PROMPT GENERATED", separatorWidth)
	console.Println(fixPrompt)
	console.Separator("=", separatorWidth)
}

func (c *USValidationCommand) validateStoryNumber(storyNumber string) error {
	matched, err := regexp.MatchString(`^\d+\.\d+$`, storyNumber)
	if err != nil {
		return fmt.Errorf("regex failed: %w", err)
	}

	if !matched {
		return pkgerrors.ErrInvalidStoryNumberFormat
	}

	return nil
}

// slugify converts a title string into a URL-friendly slug.
func slugify(title string) string {
	lower := strings.ToLower(title)

	var builder strings.Builder

	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('-')
		}
	}

	slug := builder.String()

	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	slug = strings.Trim(slug, "-")

	return slug
}
