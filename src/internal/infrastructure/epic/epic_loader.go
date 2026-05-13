package epic

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/domain/models"
	"bdd-cli/src/internal/domain/models/story"
	"bdd-cli/src/internal/infrastructure/config"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

type EpicLoader struct {
	basePath string
}

func NewEpicLoader(cfg *config.ViperConfig) *EpicLoader {
	// Get the epic path from configuration
	basePath := cfg.GetString("epics.path")

	return &EpicLoader{
		basePath: basePath,
	}
}

func (el *EpicLoader) LoadStoryFromEpic(storyNumber string) (*story.Story, error) {
	epicNum, storyIndex, err := el.parseStoryNumber(storyNumber)
	if err != nil {
		return nil, pkgerrors.ErrParseStoryNumberFailed(storyNumber, err)
	}

	epicDoc, err := el.loadEpicFile(epicNum)
	if err != nil {
		return nil, pkgerrors.ErrLoadEpicFailed(epicNum, err)
	}

	if storyIndex < 1 || storyIndex > len(epicDoc.Stories) {
		return nil, pkgerrors.ErrStoryIndexOutOfBoundsError(storyIndex, epicNum, len(epicDoc.Stories))
	}

	// Stories are 1-indexed in the story number, but 0-indexed in the slice
	targetStory := epicDoc.Stories[storyIndex-1]

	return &targetStory, nil
}

func (el *EpicLoader) parseStoryNumber(storyNumber string) (int, int, error) {
	// Parse format like "3.1" into epic=3, story=1
	var epicNum, storyNum int

	n, err := fmt.Sscanf(storyNumber, "%d.%d", &epicNum, &storyNum)
	if n != 2 || err != nil {
		return 0, 0, fmt.Errorf("invalid story number format: %w", pkgerrors.ErrInvalidStoryNumberFormatError(storyNumber))
	}

	return epicNum, storyNum, nil
}

func (el *EpicLoader) loadEpicFile(epicNum int) (*models.EpicDocument, error) {
	filename := fmt.Sprintf("epic-%02d-*.yaml", epicNum)
	pattern := filepath.Join(el.basePath, filename)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("search epic files failed: %w", pkgerrors.ErrSearchEpicFilesFailed(err))
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no epic file found: %w", pkgerrors.ErrNoEpicFileError(pattern))
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple epic files found: %w", pkgerrors.ErrMultipleEpicFilesError(pattern, matches))
	}

	epicFilePath := matches[0]

	data, err := os.ReadFile(epicFilePath)
	if err != nil {
		return nil, fmt.Errorf("read epic file failed: %w", pkgerrors.ErrReadEpicFileFailed(epicFilePath, err))
	}

	var epicDoc models.EpicDocument

	err = yaml.Unmarshal(data, &epicDoc)
	if err != nil {
		return nil, fmt.Errorf("parse epic YAML failed: %w", pkgerrors.ErrParseEpicYAMLFailed(epicFilePath, err))
	}

	return &epicDoc, nil
}
