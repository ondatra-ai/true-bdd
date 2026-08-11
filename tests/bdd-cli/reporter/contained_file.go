package reporter

import (
	"os"
	"path/filepath"
	"strings"
)

// ContainedFile resolves rel underneath base and reports whether the
// result is a readable file genuinely inside base.
//
// Every path this package opens from a run's own log goes through here.
// A log is data on disk, not a promise: the report reads whatever it
// names, so a path that is absolute, that climbs out with "..", or that
// reaches outside through a symlink would let loading a report read an
// arbitrary file and surface its contents in the UI.
//
// Symlinks are resolved on BOTH sides before comparing, because a purely
// lexical check passes a path whose every component is inside base while
// the final open follows a link straight out of it. Resolution also
// means a path that does not exist yet is reported as uncontained, which
// is the correct answer for a caller that is about to read it.
func ContainedFile(base, rel string) (string, bool) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}

	root, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", false
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil {
		return "", false
	}

	inside, err := filepath.Rel(root, resolved)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", false
	}

	return resolved, true
}

// ReadContained reads a file named relative to base, refusing anything
// that escapes it. The second result separates "not readable" from a
// file that is readable and empty — a caller listing what exists must
// not drop a zero-byte file.
func ReadContained(base, rel string) (string, bool) {
	path, ok := ContainedFile(base, rel)
	if !ok {
		return "", false
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	return string(blob), true
}
