package steps

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/reporter"
)

// ErrPathEscapesRun is returned when a step's path resolves outside the
// run's own working tree.
var ErrPathEscapesRun = errors.New("path resolves outside the run directory")

// containedPath resolves a scenario-supplied, run-relative path under the
// run's tmpdir and refuses one that leaves it.
//
// The path comes from docs/scenarios.yaml, so an escape is an authoring
// mistake rather than an attack. It is still the one mistake a harness
// must not make quietly: a step that reads outside the run directory
// returns a verdict about a file the run never touched, and the scenario
// passes or fails on evidence from somewhere else entirely. Resolution
// goes through reporter.ContainedFile, which resolves symlinks on both
// sides — a purely lexical check admits a path whose every component sits
// inside the tree while its last one links straight out.
//
// A path that does not EXIST is not an escape. The joined path is
// returned in that case so the caller's own os.Stat / os.ReadFile reports
// the absence in its own words — "it does not exist to be left alone"
// says more to a reader than "uncontained" would.
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

// missingInside vets a path that does not exist yet.
//
// Returning an unresolved path because its last component is missing is a
// hole, not a shortcut: with `escape` a symlink out of the tree,
// `escape/missing` does not exist, so a check on the leaf says nothing —
// and the caller's own os.Stat or os.ReadFile then follows `escape`
// straight out. So the nearest ancestor that DOES exist is resolved and
// required to be inside; only then is the joined path handed back.
//
// This is check-then-use, so a path created between the two is not
// covered. That is inherent to answering about a file by name, and the
// names here come from docs/scenarios.yaml rather than from anything
// racing us.
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
// produced.
//
// The glob expanded under that tmpdir, so the match is lexically inside it
// — but a matched entry can still be a symlink pointing out, which is what
// the containment check resolves. `glob` names the pattern in the failure,
// since that is what the scenario actually wrote.
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
