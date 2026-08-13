package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	storyinfra "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/story"
)

// RefineDeps bundles what `us refine` needs at the command boundary.
type RefineDeps struct {
	StoryCommonDeps

	StoryLoader *storyinfra.StoryLoader
}

// RunRefine drives `us refine`. Loads a story from docs/product/stories/,
// walks the us-refine checklist, and on convergence updates the
// story file in place.
func RunRefine(ctx context.Context, deps RefineDeps, storyNumber string, fix bool) error {
	return runStoryCommand(
		ctx, deps.StoryCommonDeps,
		storyNumber, "us refine", "us-refine", fix, false,
		func(versionMgr *fs.StoryVersionManager) func(context.Context) ([]*story.Story, error) {
			return loadStoryFromFile(deps.StoryLoader, storyNumber, versionMgr)
		},
	)
}

// loadStoryFromFile is the LoadItems factory for `us refine`. Loads
// the story from docs/product/stories/<id>-*.yaml and seeds the version
// manager.
func loadStoryFromFile(
	loader *storyinfra.StoryLoader,
	storyNumber string,
	versionMgr *fs.StoryVersionManager,
) func(ctx context.Context) ([]*story.Story, error) {
	return func(_ context.Context) ([]*story.Story, error) {
		doc, err := loader.Load(storyNumber)
		if err != nil {
			return nil, fmt.Errorf(
				"story file not found — run `true-bdd us create %s` first: %w",
				storyNumber, err,
			)
		}

		loaded := &doc.Story
		slog.Info("Story loaded", "id", loaded.ID, "title", loaded.Title)

		err = versionMgr.SaveInitialVersion(loaded)
		if err != nil {
			return nil, fmt.Errorf("failed to save initial story version: %w", err)
		}

		return []*story.Story{loaded}, nil
	}
}
