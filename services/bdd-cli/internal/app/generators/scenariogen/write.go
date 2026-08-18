package scenariogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// generatedMarker is Go's own convention for machine-written source, and
// what `.golangci.yaml`'s generated-file exclusion keys on.
//
// Matched against a TRIMMED line, so a CRLF checkout reads the same as an
// LF one and an indented marker still counts. bddgo carries its own copy
// of this convention (it is an agreement between a generator and a reader
// that may not be Go at all), and the two must accept the same files
// rather than merely look alike — so this is matched the same way, inside
// the leading comment block only. A whole-file match would treat a
// hand-written file that merely QUOTES the convention in a string literal
// as safe to overwrite.
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
const (
	dirPerm  = 0o755
	filePerm = 0o644
)

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

		got, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(file.Path)))
		if os.IsNotExist(err) {
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

// Write renders every planned file and writes the ones that changed,
// returning their repo-relative paths.
//
// Only the ones that changed: rewriting an identical file would touch
// its mtime, which is what a build cache and a file watcher key on, and
// a regeneration that rebuilt the world every time would stop being
// something anyone ran routinely.
func Write(plan *Plan, repoRoot string) ([]string, error) {
	var written []string

	for _, file := range plan.Files {
		want, err := Render(file)
		if err != nil {
			return nil, err
		}

		target := filepath.Join(repoRoot, filepath.FromSlash(file.Path))

		got, err := os.ReadFile(target)
		switch {
		case os.IsNotExist(err):
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

		err = os.MkdirAll(filepath.Dir(target), dirPerm)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", filepath.Dir(file.Path), err)
		}

		err = os.WriteFile(target, want, filePerm)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", file.Path, err)
		}

		written = append(written, file.Path)
	}

	return written, nil
}

// DriftReport renders the drifted files as the refusal prints them.
func DriftReport(drifted []Drift) string {
	lines := make([]string, 0, len(drifted))
	for _, drift := range drifted {
		lines = append(lines, fmt.Sprintf("  %s: %s", drift.Path, drift.Reason))
	}

	return strings.Join(lines, "\n")
}
