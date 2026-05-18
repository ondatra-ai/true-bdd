package commands

import (
	"context"
	"fmt"

	"bdd-cli/src/internal/app/generators/validate"
	"bdd-cli/src/internal/app/runner"
	"bdd-cli/src/internal/domain/models/story"
	"bdd-cli/src/internal/infrastructure/checklist"
	"bdd-cli/src/internal/infrastructure/fs"
	"bdd-cli/src/internal/infrastructure/input"
)

// StoryCommonDeps is the field set both `us create` and `us refine`
// need at the command boundary. CreateDeps embeds this and adds its
// own item loader (EpicLoader); RefineDeps does the same with
// StoryLoader. Sharing the common shape keeps runStoryCommand
// container-agnostic.
type StoryCommonDeps struct {
	ChecklistLoader    *checklist.ChecklistLoader
	Evaluator          *validate.ChecklistEvaluator
	FixGenerator       *validate.FixPromptGenerator
	FixApplier         *validate.FixApplier
	UserInputCollector *input.UserInputCollector
	TableRenderer      *runner.TableRenderer
	RunDir             *fs.RunDirectory
	StoriesDir         string
}

// storyLoadItemsFactory returns the LoadItems closure for one story
// command, bound to the version manager the engine will use to save
// post-fix snapshots.
type storyLoadItemsFactory func(
	versionMgr *fs.StoryVersionManager,
) func(ctx context.Context) ([]*story.Story, error)

// runStoryCommand is the shared body of `us create` and `us refine`.
// Both commands wire the engine identically once they have a
// LoadItems factory and a "writeNew vs update" toggle; this helper
// encapsulates that wiring so per-command entry points stay tiny.
func runStoryCommand(
	ctx context.Context,
	deps StoryCommonDeps,
	storyNumber, name, checklistName string,
	fix, writeNew bool,
	loadItems storyLoadItemsFactory,
) error {
	versionMgr := fs.NewStoryVersionManager(deps.RunDir, storyNumber)

	err := runner.Run(ctx, runner.Spec[*story.Story]{
		Name:          name,
		ChecklistName: checklistName,
		StoryNumber:   storyNumber,
		Fix:           fix,

		LoadItems:  loadItems(versionMgr),
		PostFix:    runner.StoryPostFix(versionMgr),
		Finalize:   runner.StoryFinalize(deps.StoriesDir, storyNumber, versionMgr, fix, writeNew),
		GetSubject: runner.StorySubject,

		Evaluator:    deps.Evaluator,
		FixGenerator: deps.FixGenerator,
		FixApplier:   deps.FixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          deps.RunDir.GetTmpOutPath(),
	})
	if err != nil {
		return fmt.Errorf("%s command failed: %w", name, err)
	}

	return nil
}
