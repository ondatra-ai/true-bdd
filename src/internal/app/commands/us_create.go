package commands

import (
	"context"
	"fmt"
	"log/slog"

	"bdd-cli/src/internal/domain/models/story"
	"bdd-cli/src/internal/infrastructure/epic"
	"bdd-cli/src/internal/infrastructure/fs"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

// CreateDeps bundles what `us create` needs at the command boundary.
type CreateDeps struct {
	StoryCommonDeps

	EpicLoader *epic.EpicLoader
}

// RunCreate drives `us create`. Loads a story from its epic, walks
// the us-create checklist, and on convergence writes the new story
// file under StoriesDir.
func RunCreate(ctx context.Context, deps CreateDeps, storyNumber string, fix bool) error {
	return runStoryCommand(
		ctx, deps.StoryCommonDeps,
		storyNumber, "us create", "us-create", fix, true,
		func(versionMgr *fs.StoryVersionManager) func(context.Context) ([]*story.Story, error) {
			return loadStoryFromEpic(deps.EpicLoader, storyNumber, versionMgr)
		},
	)
}

// loadStoryFromEpic is the LoadItems factory for `us create`. Loads
// the story from its epic and seeds the version manager with the
// initial snapshot. The engine's first `US CREATE — Story <id>`
// banner already announces the subject so we don't pretty-print the
// loaded story body here.
func loadStoryFromEpic(
	loader *epic.EpicLoader,
	storyNumber string,
	versionMgr *fs.StoryVersionManager,
) func(ctx context.Context) ([]*story.Story, error) {
	return func(_ context.Context) ([]*story.Story, error) {
		loaded, err := loader.LoadStoryFromEpic(storyNumber)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to load story: %w",
				pkgerrors.ErrLoadStoryFromEpicFailed(err),
			)
		}

		slog.Info("Story loaded", "id", loaded.ID, "title", loaded.Title)

		err = versionMgr.SaveInitialVersion(loaded)
		if err != nil {
			return nil, fmt.Errorf("failed to save initial story version: %w", err)
		}

		return []*story.Story{loaded}, nil
	}
}
