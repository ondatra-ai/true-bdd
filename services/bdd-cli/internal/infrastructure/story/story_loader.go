package story

import (
	"fmt"
	"log/slog"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// StoryLoader provides utilities for loading story information.
type StoryLoader struct {
	storiesDir string
}

// NewStoryLoader creates a new story loader.
func NewStoryLoader(cfg *config.ViperConfig) *StoryLoader {
	return &StoryLoader{
		storiesDir: cfg.GetString("paths.stories_dir"),
	}
}

// Load loads the complete story document from the file FindStoryFile
// resolves storyID to.
func (l *StoryLoader) Load(storyID string) (*story.StoryDocument, error) {
	slog.Debug("Loading story document", "story_number", storyID)

	storyFile, err := FindStoryFile(l.storiesDir, storyID)
	if err != nil {
		return nil, err
	}

	var storyDoc story.StoryDocument

	err = yaml.Unmarshal(storyFile.Data, &storyDoc)
	if err != nil {
		slog.Error("Failed to unmarshal story YAML", "file", storyFile.Path, "error", err)

		return nil, fmt.Errorf("parse story YAML failed: %w", pkgerrors.ErrParseStoryYAMLFailed(storyFile.Path, err))
	}

	// Info, not Debug: this path is the subject side of every check the
	// run performs, and the report names it when explaining what a turn
	// was comparing against what.
	slog.Info(
		"Story document loaded",
		"story_number", storyID,
		"file", storyFile.Path,
	)

	return &storyDoc, nil
}
