package lint

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
)

// ErrFailed is a gate's verdict, not a diagnosis — the gate printed its own
// findings already, so a caller must not print this on top of them.
var ErrFailed = errors.New("lint failed")

// tests/legacy/ is there to be deleted; the fixtures are host content.
//
//nolint:gochecknoglobals // a prefix list; a constant in all but syntax.
var excludedTrees = []string{"tests/bdd-cli/fixtures/", "tests/legacy/"}

func excludedTree(path string) bool {
	for _, prefix := range excludedTrees {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// trackedFiles is `git ls-files -co --exclude-standard`: tracked plus
// untracked-and-not-ignored, so a stray fails before it is ever committed.
func trackedFiles(pathspecs ...string) ([]string, error) {
	args := append([]string{"ls-files", "-co", "--exclude-standard"}, pathspecs...)

	out, err := git.Output(args...)
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	var paths []string

	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}

		info, err := os.Stat(line)
		if err == nil && info.Mode().IsRegular() {
			paths = append(paths, line)
		}
	}

	return paths, nil
}

// splitLines drops the trailing empty element a final newline produces, so
// callers can range over it as "the lines of this file".
func splitLines(text string) []string {
	if text == "" {
		return nil
	}

	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// head is the first n lines, for the cheap tests that only look at the top.
func head(src []byte, n int) []byte {
	lines := splitLines(string(src))
	if len(lines) > n {
		lines = lines[:n]
	}

	return []byte(strings.Join(lines, "\n"))
}
