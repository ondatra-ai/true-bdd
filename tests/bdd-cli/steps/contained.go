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
	// say so in its own words.
	if !present(joined) {
		return joined, nil
	}

	return "", s.fail("%w: %q", ErrPathEscapesRun, rel)
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
		// Absent: the caller reports it against the path it asked for.
		return joined, nil
	}

	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", s.fail("%w: %q", ErrPathEscapesRun, rel)
	}

	inside, err := filepath.Rel(root, resolved)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
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
