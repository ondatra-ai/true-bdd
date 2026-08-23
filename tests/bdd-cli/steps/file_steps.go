package steps

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// changeKindCreated is the diff kind fstree reports for a file the run
// brought into existence.
const changeKindCreated = "created"

// File-effect assertions read what a run did from its structural diff
// (State.Result.Diff, kind changeKindCreated/"modified"/"deleted"); a line
// count reads the file on disk instead — the diff says THAT it changed, not its content.

// assertFileCreated pins that the run created the named file: its path
// appears in the diff with kind changeKindCreated. A file modified,
// deleted, or untouched instead is named in the failure.
func assertFileCreated(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	for _, change := range state.Result.Diff {
		if change.Path != path {
			continue
		}

		if change.Kind == changeKindCreated {
			return nil
		}

		return state.fail("expected file %q to be created, but the run %s it", path, change.Kind)
	}

	return state.fail("expected file %q to be created, but the run did not create it", path)
}

// assertFileModified pins that the run modified the named file: its path
// appears in the diff with kind "modified". A file created, deleted, or
// untouched instead is named in the failure.
func assertFileModified(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	for _, change := range state.Result.Diff {
		if change.Path != path {
			continue
		}

		if change.Kind == "modified" {
			return nil
		}

		return state.fail("expected file %q to be modified, but the run %s it", path, change.Kind)
	}

	return state.fail("expected file %q to be modified, but the run left it unchanged", path)
}

// assertFileUnchanged pins that the named file is absent from the diff
// AND still exists on disk — a mistyped or nonexistent path would
// otherwise pass "unchanged" vacuously.
func assertFileUnchanged(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	for _, change := range state.Result.Diff {
		if change.Path == path {
			return state.fail("expected file %q to be unchanged, but the run %s it", path, change.Kind)
		}
	}

	full, containErr := state.containedPath(path)
	if containErr != nil {
		return containErr
	}

	_, err := os.Stat(full)
	if err != nil {
		return state.fail(
			"expected file %q to be unchanged, but it does not exist to be left alone: %v", path, err)
	}

	return nil
}

// assertFileLineCount pins the final line count of the named file. It is
// how a scenario proves a fix loop re-ran only the test it fixed: a canary
// log gaining one line per suite run must hold exactly one after a narrowed rerun.
func assertFileLineCount(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("line count %q is not a number: %w", args[1], err)
	}

	full, containErr := state.containedPath(path)
	if containErr != nil {
		return containErr
	}

	content, readErr := os.ReadFile(full)
	if readErr != nil {
		return state.fail(
			"expected file %q to have %d line(s), but it could not be read: %v", path, want, readErr)
	}

	got := countLines(content)
	if got != want {
		return state.fail("expected file %q to have %d line(s), but it has %d", path, want, got)
	}

	return nil
}

// assertFileMatches pins that the named file's content matches a regexp,
// read from disk — proof the fix loop wrote a file that says the right
// thing, e.g. a page whose markup carries the heading a browser test asserts.
func assertFileMatches(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("file-match pattern %q does not compile: %w", args[1], err)
	}

	full, containErr := state.containedPath(path)
	if containErr != nil {
		return containErr
	}

	content, readErr := os.ReadFile(full)
	if readErr != nil {
		return state.fail(
			"expected file %q to match %q, but it could not be read: %v", path, args[1], readErr)
	}

	if !pattern.Match(content) {
		return state.fail(
			"expected file %q to match %q, but its content was:\n%s", path, args[1], content)
	}

	return nil
}

// assertFilesMatchingCreated pins that the run created EXACTLY the named
// number of files matching a glob — the count assertion no single-path
// `is created` step can make when the fix loop chooses the filename.
func assertFilesMatchingCreated(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("file count %q is not a number: %w", args[0], err)
	}

	glob := args[1]

	matched, err := createdMatching(state, glob)
	if err != nil {
		return err
	}

	if len(matched) != want {
		return state.fail(
			"expected exactly %d file(s) matching %q to be created, but %d were: %v",
			want, glob, len(matched), matched)
	}

	return nil
}

// assertNoFileMatchingCreated is the negative twin of
// assertFilesMatchingCreated: the run created NO file matching a glob —
// e.g. proving a relocated registry left the old conventional path uncreated.
func assertNoFileMatchingCreated(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	glob := args[0]

	matched, err := createdMatching(state, glob)
	if err != nil {
		return err
	}

	if len(matched) > 0 {
		return state.fail(
			"expected no file matching %q to be created, but %d were: %v",
			glob, len(matched), matched)
	}

	return nil
}

// createdMatching returns the paths the run CREATED that match a glob
// (filepath.Match: "*" spans one path segment, never the separator).
// Validated once here: split in two, a malformed glob would pass both callers vacuously.
func createdMatching(state *State, glob string) ([]string, error) {
	_, matchErr := filepath.Match(glob, "")
	if matchErr != nil {
		return nil, state.fail("file-match glob %q is not a valid pattern: %w", glob, matchErr)
	}

	var matched []string

	for _, change := range state.Result.Diff {
		if change.Kind != changeKindCreated {
			continue
		}

		ok, _ := filepath.Match(glob, change.Path)
		if ok {
			matched = append(matched, change.Path)
		}
	}

	return matched, nil
}

// countLines counts the lines of text in a file's content. A trailing
// newline ends the last line rather than opening an empty one; empty
// content (or only a newline) is zero lines.
func countLines(content []byte) int {
	text := string(content)
	if text == "" {
		return 0
	}

	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return 0
	}

	return strings.Count(text, "\n") + 1
}
