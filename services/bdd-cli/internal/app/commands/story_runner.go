package commands

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/input"
)

// StoryCommonDeps is the field set both `us create` and `us refine` need at
// the command boundary. CreateDeps embeds this plus EpicLoader; RefineDeps
// embeds this plus StoryLoader — keeping runStoryCommand container-agnostic.
type StoryCommonDeps struct {
	ChecklistLoader    *checklist.ChecklistLoader
	DocResolver        *docs.Resolver
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

// runStoryCommand is the shared body of `us create` and `us refine`: both
// wire the engine identically given a LoadItems factory and a "writeNew vs
// update" toggle, so per-command entry points stay tiny.
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
		DocResolver:     deps.DocResolver,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          deps.RunDir.GetTmpOutPath(),
	})
	if err != nil {
		return fmt.Errorf("%s command failed: %w", name, err)
	}

	return nil
}
