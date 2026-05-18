package story

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/domain/models/story"
	"bdd-cli/src/internal/infrastructure/config"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
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

// Load loads the complete story document from YAML file.
func (l *StoryLoader) Load(storyNumber string) (*story.StoryDocument, error) {
	slog.Debug("Loading story document", "story_number", storyNumber)

	pattern := filepath.Join(l.storiesDir, storyNumber+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		slog.Error("Failed to glob story files", "error", err, "pattern", pattern)

		return nil, fmt.Errorf("find story file failed: %w", pkgerrors.ErrFindStoryFileFailed(err))
	}

	if len(matches) == 0 {
		slog.Error("No story file found", "story_number", storyNumber, "pattern", pattern)

		return nil, fmt.Errorf(
			"story file not found: %w",
			pkgerrors.ErrStoryFileNotFoundError(storyNumber, l.storiesDir, storyNumber),
		)
	}

	if len(matches) > 1 {
		slog.Error("Multiple story files found", "story_number", storyNumber, "count", len(matches), "files", matches)

		return nil, fmt.Errorf("multiple story files: %w", pkgerrors.ErrMultipleStoryFilesError(storyNumber, matches))
	}

	storyFile := matches[0]

	data, err := os.ReadFile(storyFile)
	if err != nil {
		slog.Error("Failed to read story file", "file", storyFile, "error", err)

		return nil, fmt.Errorf("read story file failed: %w", pkgerrors.ErrReadStoryFileFailed(storyFile, err))
	}

	var storyDoc story.StoryDocument

	err = yaml.Unmarshal(data, &storyDoc)
	if err != nil {
		slog.Error("Failed to unmarshal story YAML", "file", storyFile, "error", err)

		return nil, fmt.Errorf("parse story YAML failed: %w", pkgerrors.ErrParseStoryYAMLFailed(storyFile, err))
	}

	slog.Debug(
		"Story document loaded successfully",
		"story_number", storyNumber,
		"file", storyFile,
	)

	return &storyDoc, nil
}
