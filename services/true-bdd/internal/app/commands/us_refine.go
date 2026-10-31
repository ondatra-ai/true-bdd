package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
	storyinfra "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// RefineDeps bundles what `us refine` needs at the command boundary.
type RefineDeps struct {
	StoryLoader        *storyinfra.StoryLoader
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

// RunRefine drives `us refine`. Loads a story from docs/product/stories/,
// walks the us-refine checklist, and on convergence updates the
// story file in place.
func RunRefine(ctx context.Context, deps RefineDeps, storyID string, fix bool) error {
	versionMgr := fs.NewStoryVersionManager(deps.RunDir, storyID)

	loadItems := loadStoryFromFile(deps.StoryLoader, storyID, versionMgr)
	if fix {
		loadItems = seedRefineDocuments(loadItems, deps.DocResolver, deps.RunDir.GetTmpOutPath())
	}

	return runner.Run(ctx, runner.Spec[*template.StoryRefineData]{
		Name:          "us refine",
		ChecklistName: "us-refine",
		StoryID:       storyID,
		Fix:           fix,

		LoadItems:  loadItems,
		PostFix:    refinePostFix(versionMgr),
		Finalize:   refineFinalize(runner.StoryFinalize(deps.StoriesDir, storyID, versionMgr, fix)),
		GetSubject: refineSubject,

		Evaluator:    deps.Evaluator,
		FixGenerator: deps.FixGenerator,
		FixApplier:   deps.FixApplier,

		ChecklistLoader: deps.ChecklistLoader,
		DocResolver:     deps.DocResolver,
		Renderer:        deps.TableRenderer,
		UI:              runner.NewFixLoopUI(deps.UserInputCollector),
		TmpDir:          deps.RunDir.GetTmpOutPath(),
	})
}

// loadStoryFromFile is the LoadItems factory for `us refine`. Loads
// the story from docs/product/stories/<id>-*.yaml and seeds the version
// manager.
func loadStoryFromFile(
	loader *storyinfra.StoryLoader,
	storyID string,
	versionMgr *fs.StoryVersionManager,
) func(ctx context.Context) ([]*template.StoryRefineData, error) {
	return func(_ context.Context) ([]*template.StoryRefineData, error) {
		doc, err := loader.Load(storyID)
		if err != nil {
			// The "write it first" advice applies only when the story is
			// genuinely absent, not when Load fails on a duplicate id, an
			// unreadable file, or invalid YAML — that advice would be actively wrong for those cases.
			if errors.Is(err, pkgerrors.ErrStoryFileNotFound) {
				return nil, fmt.Errorf("story file not found — write the Story file first: %w", err)
			}

			return nil, fmt.Errorf("failed to load story %s: %w", storyID, err)
		}

		loaded := &doc.Story
		slog.Info("Story loaded", "id", loaded.ID, "title", loaded.Title)

		err = versionMgr.SaveInitialVersion(loaded)
		if err != nil {
			return nil, fmt.Errorf("failed to save initial story version: %w", err)
		}

		return []*template.StoryRefineData{refineItem(loaded, nil)}, nil
	}
}
