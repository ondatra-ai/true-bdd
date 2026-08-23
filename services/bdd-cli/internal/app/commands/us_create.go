package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/epic"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/fs"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
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

// loadStoryFromEpic is the LoadItems factory for `us create`. Loads the
// story from its epic and seeds the version manager with the initial
// snapshot; not pretty-printed here since the engine's first banner already announces the subject.
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
