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

// File-effect assertions verify what a run did to the working tree:
// which files it modified, which it left untouched, and how large a file
// it produced. The change facts come from the run's structural diff
// (State.Result.Diff — []fstree.Change whose Kind is changeKindCreated,
// "modified", or "deleted"); a line count comes from the file on disk
// under Result.TmpDir, because the diff records THAT a file changed while
// the disk holds its final content. Each path (and the line count) is a
// capture group, so one definition serves every scenario that names a
// different file or number rather than one scenario's literal line.

// assertFileCreated pins that the run created the named file: its path
// appears in the diff with kind changeKindCreated. A file that was modified,
// deleted, or never touched is named in the failure so a reader sees
// what actually happened instead. The path is a capture group so one
// definition serves every scenario that names a different file.
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
// appears in the diff with kind "modified". A file that was created,
// deleted, or never touched is named in the failure so a reader sees
// what actually happened instead.
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

// assertFileUnchanged pins that the run left the named file alone: it
// appears nowhere in the diff. The file must still exist on disk — a path
// that never existed is "unchanged" only vacuously, so a scenario naming
// a mistyped path would otherwise pass for the wrong reason.
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

	full := filepath.Join(state.Result.TmpDir, path)

	_, err := os.Stat(full)
	if err != nil {
		return state.fail(
			"expected file %q to be unchanged, but it does not exist to be left alone: %v", path, err)
	}

	return nil
}

// assertFileLineCount pins the final line count of the named file, read
// from disk under the run's tmpdir. It is how a scenario proves the fix
// loop re-ran only the test it fixed and not the whole suite: a canary
// log that gains one line per suite run must hold exactly one after a
// correctly narrowed rerun. The count is a capture group so one
// definition serves every scenario that names a different number.
func assertFileLineCount(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("line count %q is not a number: %w", args[1], err)
	}

	full := filepath.Join(state.Result.TmpDir, path)

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
// read from disk under the run's tmpdir. It is how a scenario proves the
// fix loop wrote not merely a file at a path but a file that says the
// right thing — a page whose markup carries the heading the browser test
// asserts. The path and the pattern are both capture groups so one
// definition serves every scenario that names a different file or regexp.
// The pattern runs undelimited to the end of the line, like `stdout
// matches`, so a regexp carrying its own quotes needs no escaping.
func assertFileMatches(state *State, args []string) error {
	if state.Result == nil {
		return state.fail("%w", ErrNoRun)
	}

	path := args[0]

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("file-match pattern %q does not compile: %w", args[1], err)
	}

	full := filepath.Join(state.Result.TmpDir, path)

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
// number of files whose path matches a glob (filepath.Match semantics:
// "*" spans one path segment, never the separator). It is the count
// assertion no single-path `is created` step can make — when the fix loop
// authors a step-definition file, the exact filename is the loop's to
// choose, so a scenario can only pin "one .go file appeared under this
// directory", not its name. The count and the glob are both capture
// groups so one definition serves every scenario naming a different
// number or directory.
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

// assertNoFileMatchingCreated pins that the run created NO file whose path
// matches a glob (filepath.Match semantics: "*" spans one path segment,
// never the separator). It is the negative twin of
// assertFilesMatchingCreated: a scenario uses it to prove the run wrote
// nothing at a path it must never touch — here, that relocating the
// registry to docs/specs/requirements.yaml left the OLD conventional
// docs/scenarios.yaml uncreated, so the location really did come from
// config and not a hardcoded default. The glob is a capture group so one
// definition serves every scenario naming a different path or pattern.
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
// (filepath.Match semantics: "*" spans one path segment, never the
// separator).
//
// The glob is validated before the walk, and that validation is the whole
// reason this is one function rather than two loops: a malformed glob
// makes every path silently non-matching, so the count reads zero and
// both the "exactly N" and the "no files" assertion pass vacuously. A
// guard against a FALSE PASS is the last thing that should exist in two
// copies, since a fix applied to one of them leaves the other silent.
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
// newline terminates the last line rather than opening an empty one, so
// it adds no line; empty content (or content that is only a newline) is
// zero lines.
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
