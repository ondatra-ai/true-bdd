package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/app/engine"
	"bdd-cli/src/internal/app/generators/validate"
	checklistmodels "bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/domain/models/story"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/pkg/console"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	SeparatorWidth       = 80
	storyFilePermissions = 0o644
	storyDirPermissions  = 0o755
	scratchFilePerm      = 0o644
)

// errInvalidStoryNumberFormat is the canonical error returned by
// validateStoryNumber. Wraps the package error so callers can both
// errors.Is-match and format a useful message.
var errInvalidStoryNumberFormat = errors.New("invalid story number format")

// validateStoryNumber rejects anything that isn't `<digit>.<digit>`.
func validateStoryNumber(storyNumber string) error {
	matched, err := regexp.MatchString(`^\d+\.\d+$`, storyNumber)
	if err != nil {
		return fmt.Errorf("regex failed: %w", err)
	}

	if !matched {
		return errInvalidStoryNumberFormat
	}

	return nil
}

// slugify converts a title string into a URL-friendly slug for use
// in story filenames.
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

	return strings.Trim(slug, "-")
}

// displayFailureInfo prints the section / question / rationale /
// context block for the first failed check.
func displayFailureInfo(failedCheck *checklistmodels.ValidationResult) {
	console.BlankLine()
	console.Header("CHECK FAILED: "+failedCheck.SectionPath, SeparatorWidth)
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

// displayFixPrompt prints the rendered fix prompt under a banner.
// The opening banner is enough framing — the next thing on stdout is
// the interactive apply/refine/exit prompt, which prints its own
// separators.
func displayFixPrompt(fixPrompt string) {
	console.BlankLine()
	console.Header("FIX PROMPT GENERATED", SeparatorWidth)
	console.Println(fixPrompt)
}

// writeNewStoryFile writes a fresh story YAML under `<id>-<slug>.yaml`
// and returns its path. Used by `us create` after the walk converges.
func writeNewStoryFile(storyData *story.Story, storiesDir string) (string, error) {
	slug := slugify(storyData.Title)
	filename := fmt.Sprintf("%s-%s.yaml", storyData.ID, slug)
	filePath := filepath.Join(storiesDir, filename)

	err := os.MkdirAll(storiesDir, storyDirPermissions)
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

// updateStoryFile replaces the canonical story file in place. Used by
// `us refine` after the walk converges. Falls back to creating a new
// file if no matching file is found.
func updateStoryFile(storyNumber string, updatedStory *story.Story, storiesDir string) (string, error) {
	pattern := filepath.Join(storiesDir, storyNumber+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	if len(matches) == 0 {
		return writeNewStoryFile(updatedStory, storiesDir)
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

// fixPromptGenInput bundles one call to the FixPromptGenerator and
// the metadata needed to wire its output back into the engine.
type fixPromptGenInput struct {
	generator   *validate.FixPromptGenerator
	subject     any
	subjectID   string
	failedCheck checklistmodels.ValidationResult
	tmpDir      string
	userAnswers map[string]string
	iteration   int
}

// runFixPromptGeneration adapts one call to FixPromptGenerator.Generate
// into the engine's FixResult type. Used by both us create/refine and
// us apply genFix closures so the engine doesn't import the validate
// package directly.
func runFixPromptGeneration(
	ctx context.Context,
	input fixPromptGenInput,
) (engine.FixResult, error) {
	result, err := input.generator.Generate(ctx, validate.GenerateParams{
		Subject:     input.subject,
		SubjectID:   input.subjectID,
		FailedCheck: input.failedCheck,
		TmpDir:      input.tmpDir,
		UserAnswers: input.userAnswers,
		Iteration:   input.iteration,
	})
	if err != nil {
		return engine.FixResult{}, pkgerrors.ErrFixPromptGenerationFailed(err)
	}

	out := engine.FixResult{FixPrompt: result.FixPrompt}

	if result.HasQuestions() {
		out.Questions = make([]engine.ClarifyQuestion, 0, len(result.Questions))

		for _, question := range result.Questions {
			out.Questions = append(out.Questions, engine.ClarifyQuestion{
				ID:       question.ID,
				Question: question.Question,
				Context:  question.Context,
				Options:  question.Options,
			})
		}
	}

	return out, nil
}

// StorySubject is the GetSubject implementation shared by us create
// and us refine. Pulls the (id, title) pair the report builder uses
// for table headings.
func StorySubject(item *story.Story) (string, string) {
	return item.ID, item.Title
}

// StoryPostFix returns the PostFix closure for story-based commands.
// The FixApplier returns the full updated story body as YAML; this
// closure unmarshals it, pins the canonical ID, saves a new version,
// and returns the freshly loaded latest snapshot — which the engine
// uses for the next Query iteration against the same item.
//
// The applier's contract is to emit every story field (top-level
// `title`/`as_a`/`i_want`/`so_that`/`status` plus `acceptance_criteria`)
// so any fix — not just AC-shaped ones — actually lands. The story ID
// is reasserted from the in-memory item to defend against an applier
// that drops or rewrites it.
func StoryPostFix(
	versionMgr *fs.StoryVersionManager,
) func(ctx context.Context, item *story.Story, applierContent string) (*story.Story, error) {
	return func(_ context.Context, item *story.Story, applierContent string) (*story.Story, error) {
		var updated story.Story

		err := yaml.Unmarshal([]byte(applierContent), &updated)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated story body: %w", err)
		}

		updated.ID = item.ID

		_, err = versionMgr.SaveNextVersion(&updated)
		if err != nil {
			return nil, pkgerrors.ErrSaveStoryVersionFailed(err)
		}

		console.Printf(
			"\nFix applied (v%d) — re-running validation...\n",
			versionMgr.GetCurrentVersion(),
		)

		latest, err := versionMgr.LoadLatest()
		if err != nil {
			return nil, fmt.Errorf("failed to load latest version: %w", err)
		}

		return latest, nil
	}
}

// StoryFinalize returns the Finalize closure for story-based commands.
// On Converged it writes the final story file (new or update toggle);
// every other stop reason prints a help message and returns nil.
func StoryFinalize(
	storiesDir, storyNumber string,
	versionMgr *fs.StoryVersionManager,
	fix, writeNew bool,
) func(*engine.Result[*story.Story]) error {
	return func(result *engine.Result[*story.Story]) error {
		switch result.Reason {
		case engine.Converged:
			return writeConvergedStory(versionMgr, storiesDir, storyNumber, writeNew)
		case engine.NotFixed:
			console.BlankLine()

			if fix {
				console.Println("Validation failed. No fix was applied.")
			} else {
				console.Println("Validation failed. Use --fix flag to enter interactive fix mode.")
			}

			return nil
		case engine.UserExit:
			console.Printf(
				"\nExiting. Latest version saved at: %s\n",
				versionMgr.GetLatestPath(),
			)

			return nil
		case engine.MaxAttemptsExhausted:
			console.Println("Hit max apply attempts without convergence.")

			return nil
		}

		return nil
	}
}

// writeConvergedStory writes the engine's final item to disk. The
// new-vs-update toggle reflects whether the command is creating a
// brand-new story file or updating an existing one.
func writeConvergedStory(
	versionMgr *fs.StoryVersionManager,
	storiesDir, storyNumber string,
	writeNew bool,
) error {
	console.Header("ALL CHECKS PASSED!", SeparatorWidth)

	latest, err := versionMgr.LoadLatest()
	if err != nil {
		slog.Warn("Could not load latest story for writing", "error", err)

		return nil
	}

	var storyPath string

	if writeNew {
		storyPath, err = writeNewStoryFile(latest, storiesDir)
	} else {
		storyPath, err = updateStoryFile(storyNumber, latest, storiesDir)
	}

	if err != nil {
		slog.Warn("Could not write story file", "error", err)
		console.Printf("Warning: Could not write story file: %v\n", err)

		return nil
	}

	console.Printf("Story saved to: %s\n", storyPath)

	return nil
}

// CopyFile makes a byte-for-byte copy of src at dst, creating dst's
// parent directory if needed. Used by `us apply` to seed the scratch
// requirements registry.
func CopyFile(src, dst string) error {
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

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, scratchFilePerm)
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
