package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/console"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	storyinfra "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/story"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

const (
	SeparatorWidth       = 80
	storyFilePermissions = 0o644
	storyDirPermissions  = 0o755
	scratchFilePerm      = 0o644
)

// errIncompleteGeneratorTriple is the canonical error returned by
// validateGenerators.
var errIncompleteGeneratorTriple = errors.New(
	"command is wired without its full generator triple")

// validateGenerators refuses a Spec missing a generator the engine's
// closures dereference unconditionally, so a command wired with a nil
// one is refused at startup rather than panicking mid-walk.
func validateGenerators[I any](spec Spec[I]) error {
	missing := make([]string, 0)

	if spec.Evaluator == nil {
		missing = append(missing, "Evaluator")
	}

	if spec.FixGenerator == nil {
		missing = append(missing, "FixGenerator")
	}

	if spec.FixApplier == nil {
		missing = append(missing, "FixApplier")
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", errIncompleteGeneratorTriple, strings.Join(missing, ", "))
}

// storyIDSlug is the shape of every Story id (ADR 0013): `scen-check` and
// `96.2` both pass, and an id carrying a `/` never reaches the stories glob.
var storyIDSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

// errInvalidStoryID is the canonical error returned by validateStoryID.
var errInvalidStoryID = errors.New("invalid story id")

// validateStoryID refuses a Story id that is not a safe slug, naming the
// id: this is the user-facing message itself, not one a wrapper elaborates.
func validateStoryID(storyID string) error {
	if !storyIDSlug.MatchString(storyID) {
		return fmt.Errorf("%w %q: expected a slug of lower-case letters, digits, dots and hyphens, "+
			"starting with a letter or digit", errInvalidStoryID, storyID)
	}

	return nil
}

// errChecklistHasNoPrompts is the canonical error returned by
// validateChecklistHasPrompts, kept free of regex metacharacters since
// fixtures pin it with an undelimited `stdout matches` pattern.
var errChecklistHasNoPrompts = errors.New(
	"checklist has no prompts, so every item would pass without being checked")

// validateChecklistHasPrompts refuses a walk whose checklist flattened to
// nothing: an empty prompt set satisfies every later check vacuously, and
// the walk converges having asked no question of any item.
func validateChecklistHasPrompts(
	checklistName string,
	prompts []checklistmodels.PromptWithContext,
) error {
	if len(prompts) > 0 {
		return nil
	}

	return fmt.Errorf("%s: %w", checklistName, errChecklistHasNoPrompts)
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

	paths, err := resolver.ResolveAll(keys)
	if err != nil {
		return fmt.Errorf("%w: %w", errUnsatisfiableChecklistDocs, err)
	}

	if path := paths[docs.KeyArchitectureYAML]; path != "" {
		_, err = architecture.LoadInterfaces(path, ".")
		if err != nil {
			return fmt.Errorf("%w: %w", errUnsatisfiableChecklistDocs, err)
		}
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

		// The label goes in unquoted: half of them carry double
		// quotes of their own, and %q would render those as \" — an
		// escaping scheme in a line whose whole job is to be read.
		offenders = append(offenders, fmt.Sprintf(
			"%s #%d %s",
			prompts[idx].GetFullSectionPath(),
			prompts[idx].Index,
			prompts[idx].Prompt.Label(),
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

// reported wraps an error already diagnosed by the code that produced
// it — Error()/Unwrap() pass straight through, so the marker is
// invisible to everything except RefuseStartup.
type reported struct{ error }

// Unwrap keeps errors.Is/As working through the marker.
func (r reported) Unwrap() error { return r.error }

// Reported marks an error as already diagnosed, so the generic startup
// refusal stays silent about it — build code's LoadItems already prints
// "Cannot run <svc>/<layer>:" itself before returning.
func Reported(err error) error { return reported{err} }

// RefuseStartup reports a precondition failure on both stdout and the
// log, the way validateRequiredDocs and cmd.refuseUnresolvedDoc do —
// a bare cobra stderr usage dump reaches neither a harness nor the judge.
func RefuseStartup(command string, err error) error {
	var alreadyReported reported
	if errors.As(err, &alreadyReported) {
		return err
	}

	slog.Error("Refusing to start",
		"command", command,
		"error", err,
	)

	return err
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

// errStoryFileGone is returned by updateStoryFile when the file the walk
// loaded no longer matches its id: rewriting under a guessed name would
// leave the Story in two files, or in one nobody asked for.
var errStoryFileGone = errors.New("the story file the walk loaded is gone")

// updateStoryFile replaces the story file `us refine` loaded, in place,
// after the walk converges: the file storyinfra.FindStoryFile resolves the
// id to, as the load did.
func updateStoryFile(storyID string, updatedStory *story.Story, storiesDir string) (string, error) {
	storyFile, err := storyinfra.FindStoryFile(storiesDir, storyID)
	if errors.Is(err, pkgerrors.ErrStoryFileNotFound) {
		return "", pkgerrors.ErrWriteStoryFileFailed(fmt.Errorf("%w: %w", errStoryFileGone, err))
	}

	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	filePath := storyFile.Path

	wrapper := struct {
		Story story.Story `yaml:"story"`
	}{Story: *updatedStory}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return "", pkgerrors.ErrWriteStoryFileFailed(err)
	}

	err = disk.Write(filePath, data, disk.Shared)
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
// into the engine's FixResult type, so us refine's and us apply's
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

// StoryPostFix returns the PostFix closure for story-based commands: the
// applier's YAML lands on a zero Story, zeroing what it omits, so the ID and
// the Feature label (its template never shows it) are reasserted from the item.
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
		updated.Feature = item.Feature

		_, err = versionMgr.SaveNextVersion(&updated)
		if err != nil {
			return nil, pkgerrors.ErrSaveStoryVersionFailed(err)
		}

		slog.Info("Fix applied; re-running validation",
			"version", versionMgr.GetCurrentVersion())

		latest, err := versionMgr.LoadLatest()
		if err != nil {
			return nil, fmt.Errorf("failed to load latest version: %w", err)
		}

		return latest, nil
	}
}

// StoryFinalize returns the Finalize closure for story-based commands.
// On Converged it rewrites the story file in place; every other stop
// reason prints a help message and returns nil.
func StoryFinalize(
	storiesDir, storyID string,
	versionMgr *fs.StoryVersionManager,
	fix bool,
) func(*engine.Result[*story.Story]) error {
	return func(result *engine.Result[*story.Story]) error {
		switch result.Reason {
		case engine.Converged:
			return writeConvergedStory(versionMgr, storiesDir, storyID)
		case engine.NotFixed:
			if fix {
				slog.Warn("Validation failed. No fix was applied.")
			} else {
				slog.Warn("Validation failed. Use --fix flag to enter interactive fix mode.")
			}

			return nil
		case engine.UserExit:
			slog.Info("Exiting", "latest_version", versionMgr.GetLatestPath())

			return nil
		case engine.MaxAttemptsExhausted:
			slog.Warn("Hit max apply attempts without convergence.")

			return nil
		}

		return nil
	}
}

// writeConvergedStory writes the engine's final item over its story file
// and propagates any write/load failure instead of swallowing it (plan
// §3.2): a story that never landed no longer reports success.
func writeConvergedStory(
	versionMgr *fs.StoryVersionManager,
	storiesDir, storyID string,
) error {
	slog.Info("ALL CHECKS PASSED!")

	latest, err := versionMgr.LoadLatest()
	if err != nil {
		slog.Error("Could not load latest story for writing", "error", err)

		return fmt.Errorf("load latest story for writing: %w", err)
	}

	storyPath, err := updateStoryFile(storyID, latest, storiesDir)
	if err != nil {
		slog.Error("Could not write story file", "error", err)

		return fmt.Errorf("write story file: %w", err)
	}

	slog.Info("Story saved", "path", storyPath)

	return nil
}

// CopyFile makes a byte-for-byte copy of src at dst, creating dst's
// parent directory if needed. Used by `us apply` to seed the scratch
// requirements registry.
func CopyFile(src, dst string) error {
	return disk.Copy(dst, src, disk.Shared)
}
