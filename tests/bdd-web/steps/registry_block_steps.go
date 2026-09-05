package steps

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoRegistryBlock is returned when a registry document declares no entry
// under the id a clause names.
var ErrNoRegistryBlock = errors.New("the registry declares no such scenario")

// ErrNoTreeSnapshot is returned when a byte-for-byte clause runs and the tree's
// Given recorded no reading of that document to compare against.
var ErrNoTreeSnapshot = errors.New("the project tree's documents were never snapshotted")

// registerRegistryBlockSteps binds the clause a merge scenario closes with: an
// entry the run was not asked about must come out of it byte for byte.
func registerRegistryBlockSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the scenario "([^"]+)" in "([^"]+)" is unchanged byte for byte$`,
		assertRegistryBlockUnchanged)
}

// assertRegistryBlockUnchanged holds one registry entry to the bytes it had
// before the run: a merge that rewrote a neighbour did not merge, it replaced.
func assertRegistryBlockUnchanged(state *State, args []string) error {
	scenarioID, relPath := args[0], args[1]

	raw, listed := state.TreeDocsBefore[relPath]
	if !listed {
		return state.fail("%w: %s", ErrNoTreeSnapshot, relPath)
	}

	before, err := registryBlock(raw, scenarioID)
	if err != nil {
		return state.fail("%s as the Given materialized it: %w", relPath, err)
	}

	return compareRegistryBlock(state, scenarioID, relPath, before)
}

// compareRegistryBlock reads the document as it stands now and grades the entry
// against the reading the Given took.
func compareRegistryBlock(state *State, scenarioID, relPath, before string) error {
	now, err := fixtureFile(state, relPath)
	if err != nil {
		return err
	}

	after, err := registryBlock(now, scenarioID)
	if err != nil {
		return state.fail("%s after the run: %w", relPath, err)
	}

	if after != before {
		return state.fail("scenario %s of %s now reads %s, want the bytes it had: %s",
			scenarioID, relPath, snippetOf(after), snippetOf(before))
	}

	return nil
}

// registryBlock is the slice of a registry declaring one scenario: its `<id>:`
// line down to the next line indented no deeper. Trailing blank lines are cut —
// they are the file's layout between entries, not the entry's own bytes.
func registryBlock(raw, scenarioID string) (string, error) {
	lines := strings.Split(raw, "\n")
	// Compiled per call: a package-level regexp is a global.
	opener := regexp.MustCompile(`^(\s*)` + regexp.QuoteMeta(scenarioID) + `:\s*(?:#.*)?$`)

	for index, line := range lines {
		match := opener.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		block := strings.Join(lines[index:entryEnd(lines, index, len(match[1]))], "\n")

		return strings.TrimRight(block, " \t\n"), nil
	}

	return "", fmt.Errorf("%w: %s", ErrNoRegistryBlock, scenarioID)
}

// entryEnd is the line the entry opened at start runs to: the next non-blank
// line indented no deeper than its key, or the end of the document.
func entryEnd(lines []string, start, indent int) int {
	for index := start + 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "" {
			continue
		}

		if len(lines[index])-len(strings.TrimLeft(lines[index], " \t")) <= indent {
			return index
		}
	}

	return len(lines)
}
