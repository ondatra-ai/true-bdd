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

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/console"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
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
		// Names the offending value: the bare message alone doesn't say
		// which input failed, and this is the user-facing refusal itself,
		// not an internal error a wrapper elaborates later.
		return fmt.Errorf("%w: expected <epic>.<story>, got %q",
			errInvalidStoryNumberFormat, storyNumber)
	}

	return nil
}

// errUnsatisfiableChecklistDocs is the canonical error returned by
// validateRequiredDocs.
var errUnsatisfiableChecklistDocs = errors.New(
	"checklist declares documents that cannot be provided")

// validateRequiredDocs refuses a walk whose prompts declare `docs:` paths
// that don't exist: nothing downstream verifies them once turned into
// Read() instructions, so a missing doc used to silently degrade output.
func validateRequiredDocs(
	prompts []checklistmodels.PromptWithContext,
	resolver *docs.Resolver,
) error {
	keys := make([]string, 0)

	for i := range prompts {
		// GetEffectiveDocs already applies "prompt docs: else checklist
		// default_docs:", and the caller has already dropped skip:-ped
		// prompts — so this is exactly the set that would be injected.
		keys = append(keys, prompts[i].GetEffectiveDocs()...)
	}

	if len(keys) == 0 {
		return nil
	}

	_, err := resolver.ResolveAll(keys)
	if err != nil {
		return fmt.Errorf("%w: %w", errUnsatisfiableChecklistDocs, err)
	}

	return nil
}

// errChecklistNotFixable is the canonical error returned by
// validateFixTemplates, kept in plain prose (no regex metachars) since
// fixtures pin it via an unescaped `stdout matches` pattern.
var errChecklistNotFixable = errors.New(
	"checklist has prompts with no F fix template, so --fix has nothing to apply")

// validateFixTemplates refuses a --fix run whose checklist cannot guide
// its own fixes: a missing `F:` leaves ValidationResult.FixPrompt empty,
// so the walk can only ever land on CellFailedNoFix for that cell.
func validateFixTemplates(
	fix bool,
	checklistName string,
	prompts []checklistmodels.PromptWithContext,
) error {
	if !fix {
		return nil
	}

	offenders := make([]string, 0)

	for idx := range prompts {
		if strings.TrimSpace(prompts[idx].Prompt.FixTemplate) != "" {
			continue
		}

		// The question goes in unquoted: half of them carry double
		// quotes of their own, and %q would render those as \" — an
		// escaping scheme in a line whose whole job is to be read.
		offenders = append(offenders, fmt.Sprintf(
			"%s #%d %s",
			prompts[idx].GetFullSectionPath(),
			idx+1,
			questionFirstLine(prompts[idx].Prompt.Question),
		))
	}

	if len(offenders) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s has %d of %d — %s",
		errChecklistNotFixable,
		checklistName,
		len(offenders),
		len(prompts),
		strings.Join(offenders, "; "),
	)
}

// questionFirstLine returns the first non-blank line of a prompt's Q, so
// a multi-line question still names its prompt in one line of output.
func questionFirstLine(question string) string {
	for line := range strings.SplitSeq(question, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// reported wraps an error already diagnosed by the code that produced
// it — Error()/Unwrap() pass straight through, so the marker is
// invisible to everything except refuseStartup.
type reported struct{ error }

// Unwrap keeps errors.Is/As working through the marker.
func (r reported) Unwrap() error { return r.error }

// Reported marks an error as already diagnosed, so the generic startup
// refusal stays silent about it — build code's LoadItems already prints
// "Cannot run <svc>/<layer>:" itself before returning.
func Reported(err error) error { return reported{err} }

// refuseStartup reports a precondition failure on both stdout and the
// log, the way validateRequiredDocs and cmd.refuseUnresolvedDoc do —
// a bare cobra stderr usage dump reaches neither a harness nor the judge.
func refuseStartup(command string, err error) error {
	var alreadyReported reported
	if errors.As(err, &alreadyReported) {
		return err
	}

	slog.Error("Refusing to start",
		"command", command,
		"error", err,
	)
	console.Println("Cannot start: " + err.Error())

	return err
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

// displayFixPrompt prints the rendered fix prompt under a banner; no
// closing banner is needed since the next stdout output (the
// interactive apply/refine/exit prompt) prints its own separators.
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
// into the engine's FixResult type, so us create/refine and us apply's
// genFix closures don't need the engine to import validate directly.
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

// StoryPostFix returns the PostFix closure for story-based commands: it
// unmarshals the applier's YAML onto a zero Story, so a partial body
// zeroes whatever it omits — only the ID is reasserted here as a backstop.
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

// writeConvergedStory writes the engine's final item to disk (new file
// or update, per writeNew) and propagates any write/load failure instead
// of swallowing it (plan §3.2): a story that never landed no longer reports success.
func writeConvergedStory(
	versionMgr *fs.StoryVersionManager,
	storiesDir, storyNumber string,
	writeNew bool,
) error {
	console.Header("ALL CHECKS PASSED!", SeparatorWidth)

	latest, err := versionMgr.LoadLatest()
	if err != nil {
		console.Printf("Error: could not load latest story for writing: %v\n", err)

		return fmt.Errorf("load latest story for writing: %w", err)
	}

	var storyPath string

	if writeNew {
		storyPath, err = writeNewStoryFile(latest, storiesDir)
	} else {
		storyPath, err = updateStoryFile(storyNumber, latest, storiesDir)
	}

	if err != nil {
		console.Printf("Error: could not write story file: %v\n", err)

		return fmt.Errorf("write story file: %w", err)
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
