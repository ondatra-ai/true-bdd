package story

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// StoryFile is the Story file an id resolved to: its path, and the bytes
// read to tell it apart from the other files its glob matched.
type StoryFile struct {
	Path string
	Data []byte
}

// storyIDProbe decodes a Story file's own id and nothing else, so what a
// sibling holds besides its id never decides which file an id resolves to.
type storyIDProbe struct {
	Story struct {
		ID string `yaml:"id"`
	} `yaml:"story"`
}

// declaredID is one matched file and the id it declares.
type declaredID struct {
	path string
	id   string
}

// FindStoryFile resolves storyID to the one `<storyID>-*.yaml` file whose own
// `id:` is storyID: the glob for `scen-check` also matches the Story
// `scen-check-phrases`. `us refine` and `us apply` both find a Story here.
func FindStoryFile(storiesDir, storyID string) (StoryFile, error) {
	matches, err := globStoryFiles(storiesDir, storyID)
	if err != nil {
		return StoryFile{}, err
	}

	var (
		picked []StoryFile
		others []declaredID
	)

	for _, match := range matches {
		file, declared, readErr := readStoryID(match)
		if readErr != nil {
			return StoryFile{}, readErr
		}

		if declared != storyID {
			others = append(others, declaredID{path: match, id: declared})

			continue
		}

		picked = append(picked, file)
	}

	switch len(picked) {
	case 1:
		return picked[0], nil
	case 0:
		return StoryFile{}, refuseOtherIDs(storyID, others)
	default:
		return StoryFile{}, refuseSeveralFiles(storyID, picked)
	}
}

// globStoryFiles lists the files `<storyID>-*.yaml` matches, refusing none.
func globStoryFiles(storiesDir, storyID string) ([]string, error) {
	pattern := filepath.Join(storiesDir, storyID+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		slog.Error("Failed to glob story files", "error", err, "pattern", pattern)

		return nil, fmt.Errorf("find story file failed: %w", pkgerrors.ErrFindStoryFileFailed(err))
	}

	if len(matches) == 0 {
		slog.Error("No story file found", "story_number", storyID, "pattern", pattern)

		return nil, fmt.Errorf(
			"story file not found: %w",
			pkgerrors.ErrStoryFileNotFoundError(storyID, storiesDir, storyID),
		)
	}

	return matches, nil
}

// readStoryID reads one matched file and the id it declares. A file that
// cannot be read or decoded is refused: it might be the Story asked for.
func readStoryID(path string) (StoryFile, string, error) {
	data, err := disk.Read(path)
	if err != nil {
		slog.Error("Failed to read story file", "file", path, "error", err)

		return StoryFile{}, "", fmt.Errorf("read story file failed: %w", pkgerrors.ErrReadStoryFileFailed(path, err))
	}

	var probe storyIDProbe

	err = yaml.Unmarshal(data, &probe)
	if err != nil {
		slog.Error("Failed to unmarshal story YAML", "file", path, "error", err)

		return StoryFile{}, "", fmt.Errorf("parse story YAML failed: %w", pkgerrors.ErrParseStoryYAMLFailed(path, err))
	}

	return StoryFile{Path: path, Data: data}, probe.Story.ID, nil
}

// refuseOtherIDs names every matched file and the id it declares instead.
func refuseOtherIDs(storyID string, others []declaredID) error {
	refusals := make([]error, 0, len(others))

	for _, other := range others {
		slog.Error("Story file declares another id",
			"story_number", storyID, "file", other.path, "declared_id", other.id)

		refusals = append(refusals, pkgerrors.ErrStoryIDMismatchError(storyID, other.path, other.id))
	}

	return fmt.Errorf("story id mismatch: %w", errors.Join(refusals...))
}

// refuseSeveralFiles names every file that declares storyID.
func refuseSeveralFiles(storyID string, picked []StoryFile) error {
	paths := make([]string, 0, len(picked))
	for _, file := range picked {
		paths = append(paths, file.Path)
	}

	slog.Error("Multiple story files found", "story_number", storyID, "count", len(paths), "files", paths)

	return fmt.Errorf("multiple story files: %w", pkgerrors.ErrMultipleStoryFilesError(storyID, paths))
}
