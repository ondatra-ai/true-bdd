package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrMalformedLineage is returned when a step names a coverage id that is
// not of the form <internal-id>-NNN.
var ErrMalformedLineage = errors.New("not an <internal-id>-NNN lineage id")

// ErrNoStoryFile is returned when the stories directory resolves the
// lineage's story to anything other than exactly one file.
var ErrNoStoryFile = errors.New("the stories directory resolves no single story file")

const (
	// engineConfigRel is the host's engine configuration, read for the one
	// path this file needs.
	engineConfigRel = "true-bdd/true-bdd.yaml"
	// defaultStoriesRel is the engine's own fallback when the configuration
	// declares no stories_dir.
	defaultStoriesRel = "docs/product/stories"
)

// appendCoveringScenario writes a registry entry covering one lineage id
// into the project tree — the change made OUTSIDE the browser that a
// refresh must pick up. args[0] is the role, discarded as openPath does.
func appendCoveringScenario(state *State, args []string) error {
	lineageID, registryRel := args[1], args[2]

	before, err := fixtureFile(state, registryRel)
	if err != nil {
		return err
	}

	storyRel, err := storyPathForLineage(state, lineageID)
	if err != nil {
		return err
	}

	entry := registryEntry(state.Scenario.ID, storyRel, lineageID)

	// The entry extends the registry's trailing scenarios map, so it only
	// lands as a sibling key when the file already ends a line.
	if !strings.HasSuffix(before, "\n") {
		entry = "\n" + entry
	}

	err = disk.Append(filepath.Join(state.Tree.Dir, registryRel), []byte(entry), disk.Shared)
	if err != nil {
		return state.fail("appending a scenario covering %q to %s: %w",
			lineageID, registryRel, err)
	}

	return nil
}

// registryEntry renders the appended entry. The scanner reads only the
// story path and the lineage id off it, so nothing else is stated.
func registryEntry(scenarioID, storyRel, lineageID string) string {
	return fmt.Sprintf(`  E2E-COVER-%s:
    description: "Coverage appended by %s"
    user_stories:
      - story: %q
        scenario_id: %q`, lineageID, scenarioID, storyRel, lineageID)
}

// storyPathForLineage is the registry-shaped path of the story a lineage id
// belongs to: <stories_dir>/<internal-id>-*.yaml, which must resolve to
// exactly one file, as the engine's own resolver requires.
func storyPathForLineage(state *State, lineageID string) (string, error) {
	cut := strings.LastIndex(lineageID, "-")
	if cut <= 0 {
		return "", state.fail("%w: %q", ErrMalformedLineage, lineageID)
	}

	storiesRel, err := storiesDir(state)
	if err != nil {
		return "", err
	}

	pattern := filepath.Join(state.Tree.Dir, storiesRel, lineageID[:cut]+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", state.fail("globbing %s: %w", pattern, err)
	}

	if len(matches) != 1 {
		return "", state.fail("%w: %s matched %d files, want exactly 1",
			ErrNoStoryFile, pattern, len(matches))
	}

	return filepath.ToSlash(filepath.Join(storiesRel, filepath.Base(matches[0]))), nil
}

// storiesDir is the host's configured stories directory, or the engine's
// default when the configuration declares none.
func storiesDir(state *State) (string, error) {
	dir, err := fixtureField(state, engineConfigRel, "stories_dir")
	if errors.Is(err, ErrNoFixtureField) {
		return defaultStoriesRel, nil
	}

	if err != nil {
		return "", err
	}

	return dir, nil
}
