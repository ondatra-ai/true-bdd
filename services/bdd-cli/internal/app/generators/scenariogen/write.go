package scenariogen

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// generatedMarker is Go's own convention for machine-written source;
// bddgo carries its own copy and the two must accept the same files.
// Matched only in the leading comment block, not whole-file, so a hand-written file merely quoting it stays safe.
var generatedMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// hasGeneratedMarker reports whether the marker appears in the file's
// leading comment block, which is the only place Go's convention puts it.
// Mirrors bddgo's reader exactly.
func hasGeneratedMarker(data []byte) bool {
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			if generatedMarker.MatchString(trimmed) {
				return true
			}

			continue
		}

		return false
	}

	return false
}

// dirPerm and filePerm match what the rest of the engine writes with.
const ()

// Drift is one file whose contents are not what the registry renders.
type Drift struct {
	// Path is repo-relative.
	Path string
	// Reason says which kind of disagreement this is, for the refusal
	// line: missing, or present with different contents.
	Reason string
}

// Verify reports every planned file that is absent or stale, without
// writing anything. This is what `build tests` runs without --fix.
func Verify(plan *Plan, repoRoot string) ([]Drift, error) {
	var drifted []Drift

	for _, file := range plan.Files {
		want, err := Render(file)
		if err != nil {
			return nil, err
		}

		got, err := disk.Read(filepath.Join(repoRoot, filepath.FromSlash(file.Path)))
		if errors.Is(err, fs.ErrNotExist) {
			drifted = append(drifted, Drift{Path: file.Path, Reason: "no generated test file"})

			continue
		}

		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Path, err)
		}

		if !bytes.Equal(got, want) {
			drifted = append(drifted, Drift{
				Path:   file.Path,
				Reason: "the generated test file is not what the registry renders",
			})
		}
	}

	return drifted, nil
}

// Write renders every planned file and writes only the ones that changed,
// returning their repo-relative paths. Skips identical files because
// rewriting them touches mtime, which a build cache and file watcher key on.
func Write(plan *Plan, repoRoot string) ([]string, error) {
	pending, err := pendingWrites(plan, repoRoot)
	if err != nil {
		return nil, err
	}

	written := make([]string, 0, len(pending))

	for _, file := range pending {
		target := filepath.Join(repoRoot, filepath.FromSlash(file.path))

		err = disk.Dir(filepath.Dir(target), disk.Shared)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", filepath.Dir(file.path), err)
		}

		err = disk.Write(target, file.content, disk.Shared)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", file.path, err)
		}

		written = append(written, file.path)
	}

	return written, nil
}

// renderedFile is one planned file whose rendered bytes differ from disk.
type renderedFile struct {
	path    string
	content []byte
}

// pendingWrites renders every planned file and keeps those that differ from
// disk. Every target is checked before Write writes one, so a hand-written
// file anywhere in the plan leaves even the stale generated files untouched.
func pendingWrites(plan *Plan, repoRoot string) ([]renderedFile, error) {
	var pending []renderedFile

	for _, file := range plan.Files {
		want, err := Render(file)
		if err != nil {
			return nil, err
		}

		got, err := disk.Read(filepath.Join(repoRoot, filepath.FromSlash(file.Path)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return nil, fmt.Errorf("read %s: %w", file.Path, err)
		case bytes.Equal(got, want):
			continue
		case !hasGeneratedMarker(got):
			// A typo'd `path:` pointing at a step definition would
			// otherwise delete it, and the definitions are the one thing
			// here no regeneration can reproduce.
			return nil, fmt.Errorf("%s: %w", file.Path, ErrWouldClobberHandWritten)
		}

		pending = append(pending, renderedFile{path: file.Path, content: want})
	}

	return pending, nil
}

// DriftReport renders the drifted files as the refusal prints them.
func DriftReport(drifted []Drift) string {
	lines := make([]string, 0, len(drifted))
	for _, drift := range drifted {
		lines = append(lines, fmt.Sprintf("  %s: %s", drift.Path, drift.Reason))
	}

	return strings.Join(lines, "\n")
}
