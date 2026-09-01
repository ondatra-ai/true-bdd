package steps

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrNoFixtureField is returned when the document a clause names as its
// oracle carries no such field.
var ErrNoFixtureField = errors.New("the fixture file has no such field")

// ErrNoStoryBlock is returned when the epic document declares no story
// with the id a clause names.
var ErrNoStoryBlock = errors.New("the fixture file declares no such story")

// fixtureFile reads a project-relative file out of the tree the scenario's
// Given step materialized. The expected value is READ from the fixture, so
// a fixture edit can never silently drift from the assertion.
func fixtureFile(state *State, relPath string) (string, error) {
	if state.Tree == nil {
		return "", state.fail("%w", ErrNoProjectTree)
	}

	data, err := disk.Read(filepath.Join(state.Tree.Dir, relPath))
	if err != nil {
		return "", state.fail("read the oracle %s: %w", relPath, err)
	}

	return string(data), nil
}

// fixtureField is the one-call form: a scalar field of a fixture file.
func fixtureField(state *State, relPath, field string) (string, error) {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return "", err
	}

	value, err := scalarField(raw, field)
	if err != nil {
		return "", state.fail("%s: %w", relPath, err)
	}

	return value, nil
}

// scalarField is the unquoted value of `<field>:` on its own line.
func scalarField(raw, field string) (string, error) {
	// Compiled per call: a package-level regexp is a global.
	line := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(field) + `:\s*(.+?)\s*$`)

	match := line.FindStringSubmatch(raw)
	if match == nil {
		return "", fmt.Errorf("%w: %s", ErrNoFixtureField, field)
	}

	return unquote(match[1]), nil
}

// storyBlock is the slice of an epic document declaring one story: its
// `- id: <declaredID>` line up to the next story declaration or the end,
// so a clause reads the story it named rather than the file's first.
func storyBlock(raw, declaredID string) (string, error) {
	lines := strings.Split(raw, "\n")
	opener := regexp.MustCompile(
		`^\s*-\s+id:\s*["']?` + regexp.QuoteMeta(declaredID) + `["']?\s*$`)

	for index, line := range lines {
		if opener.MatchString(line) {
			return strings.Join(lines[index:blockEnd(lines, index)], "\n"), nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNoStoryBlock, declaredID)
}

// blockEnd is the line the story block opened at start runs to: the next
// story declaration, or the end of the document.
func blockEnd(lines []string, start int) int {
	next := regexp.MustCompile(`^\s*-\s+id:\s*["']?\d`)

	for index := start + 1; index < len(lines); index++ {
		if next.MatchString(lines[index]) {
			return index
		}
	}

	return len(lines)
}

// unquote strips one matching pair of surrounding quotes.
func unquote(value string) string {
	trimmed := strings.TrimSpace(value)

	for _, quote := range []string{`"`, `'`} {
		if strings.HasPrefix(trimmed, quote) && strings.HasSuffix(trimmed, quote) &&
			len(trimmed) > len(quote) {
			return strings.TrimSuffix(strings.TrimPrefix(trimmed, quote), quote)
		}
	}

	return trimmed
}
