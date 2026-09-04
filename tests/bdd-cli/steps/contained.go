package steps

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/reporter"
)

// ErrPathEscapesRun is returned when a step's path resolves outside the
// run's own working tree.
var ErrPathEscapesRun = errors.New("path resolves outside the run directory")

// containedPath resolves a scenario-supplied, run-relative path under the
// run's tmpdir, refusing an escape. Symlink-resolved on both sides — a
// lexical check alone would admit a path whose last component links out.
func (s *State) containedPath(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || climbsOut(rel) {
		return "", s.fail("%w: %q must be relative to the run directory", ErrPathEscapesRun, rel)
	}

	resolved, ok := reporter.ContainedFile(s.Result.TmpDir, rel)
	if ok {
		return resolved, nil
	}

	joined := filepath.Join(s.Result.TmpDir, rel)

	// Absent, rather than escaping: hand the path back and let the caller
	// say so in its own words — but only once the part of it that DOES
	// exist is known to be inside the tree.
	if !present(joined) {
		return s.missingInside(rel, joined)
	}

	return "", s.fail("%w: %q", ErrPathEscapesRun, rel)
}

// missingInside vets a path that does not exist yet: checking only its
// missing leaf says nothing if an ancestor (e.g. `escape`) is a symlink
// out of the tree, so the nearest EXISTING ancestor is resolved and checked instead.
func (s *State) missingInside(rel, joined string) (string, error) {
	root, err := filepath.EvalSymlinks(s.Result.TmpDir)
	if err != nil {
		return "", s.fail("resolve the run directory: %w", err)
	}

	for dir := filepath.Dir(joined); ; dir = filepath.Dir(dir) {
		resolved, resolveErr := filepath.EvalSymlinks(dir)
		if resolveErr == nil {
			if !within(root, resolved) {
				return "", s.fail("%w: %q", ErrPathEscapesRun, rel)
			}

			return joined, nil
		}

		// Climbed to the filesystem root without finding anything that
		// exists, which cannot be a path inside the run directory.
		if parent := filepath.Dir(dir); parent == dir {
			return "", s.fail("%w: %q", ErrPathEscapesRun, rel)
		}
	}
}

// within reports whether resolved sits inside root, both already
// symlink-resolved.
func within(root, resolved string) bool {
	inside, err := filepath.Rel(root, resolved)

	return err == nil && inside != ".." && !strings.HasPrefix(inside, ".."+string(filepath.Separator))
}

// containedDir is containedPath for a directory, which
// reporter.ContainedFile refuses by design — it admits only regular
// files, because a report reading a FIFO would block forever.
func (s *State) containedDir(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || climbsOut(rel) {
		return "", s.fail("%w: %q must be relative to the run directory", ErrPathEscapesRun, rel)
	}

	joined := filepath.Join(s.Result.TmpDir, rel)

	root, err := filepath.EvalSymlinks(s.Result.TmpDir)
	if err != nil {
		return "", s.fail("resolve the run directory: %w", err)
	}

	if !present(joined) {
		// Absent: the caller reports it against the path it asked for, once
		// the existing part of the path is known to be inside the tree.
		return s.missingInside(rel, joined)
	}

	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil || !within(root, resolved) {
		return "", s.fail("%w: %q", ErrPathEscapesRun, rel)
	}

	return resolved, nil
}

// containedMatch resolves an absolute path a glob under the run's tmpdir
// produced, applying the same symlink-escape check as containedPath.
// `glob` names the pattern (not the matched path) in any failure.
func (s *State) containedMatch(glob, match string) (string, error) {
	rel, err := filepath.Rel(s.Result.TmpDir, match)
	if err != nil {
		return "", s.fail("locating %q under the run directory: %w", glob, err)
	}

	return s.containedPath(rel)
}

// present reports whether anything exists at path. Lstat rather than Stat,
// so a dangling symlink counts as present — it is a link that resolves
// nowhere, which is a path problem to name rather than a missing file.
func present(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

// climbsOut reports whether a cleaned relative path starts by leaving its
// own root. Checked before any filesystem call so a traversal is named as
// one rather than surfacing as a missing file.
func climbsOut(rel string) bool {
	cleaned := filepath.Clean(rel)

	return cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}
