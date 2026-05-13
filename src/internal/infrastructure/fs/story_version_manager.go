package fs

import (
	"fmt"
	"log/slog"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/domain/models/story"
)

// StoryVersionManager manages versioned copies of stories in tmp directory.
// Wraps ContentVersionManager with story-specific serialization.
type StoryVersionManager struct {
	content *ContentVersionManager
}

// NewStoryVersionManager creates a new version manager for the specified story.
func NewStoryVersionManager(runDir *RunDirectory, storyID string) *StoryVersionManager {
	return &StoryVersionManager{
		content: NewContentVersionManager(runDir, storyID, "story"),
	}
}

// SaveInitialVersion saves the original story as v01.
func (m *StoryVersionManager) SaveInitialVersion(storyData *story.Story) error {
	data, err := yaml.Marshal(storyData)
	if err != nil {
		return fmt.Errorf("failed to marshal story to YAML: %w", err)
	}

	return m.content.SaveInitialVersion(data)
}

// SaveNextVersion increments version and saves the story.
func (m *StoryVersionManager) SaveNextVersion(storyData *story.Story) (string, error) {
	data, err := yaml.Marshal(storyData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal story to YAML: %w", err)
	}

	return m.content.SaveNextVersion(data)
}

// LoadLatest loads the most recent version of the story.
func (m *StoryVersionManager) LoadLatest() (*story.Story, error) {
	data, err := m.content.LoadLatest()
	if err != nil {
		return nil, err
	}

	var storyData story.Story

	err = yaml.Unmarshal(data, &storyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse story YAML: %w", err)
	}

	slog.Info("Loaded story version", "version", m.content.GetCurrentVersion())

	return &storyData, nil
}

// GetLatestPath returns the file path for the current version.
func (m *StoryVersionManager) GetLatestPath() string {
	return m.content.GetLatestPath()
}

// GetCurrentVersion returns the current version number.
func (m *StoryVersionManager) GetCurrentVersion() int {
	return m.content.GetCurrentVersion()
}
