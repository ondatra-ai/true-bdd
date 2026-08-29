package lint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/linters"
)

// The gates report their own findings and the caller only needs the verdict,
// so a failed gate is this one sentinel rather than a wrapped tool error.
var (
	// ErrFailed is a gate's verdict, not a diagnosis — the gate has already
	// printed its findings, so a caller must not print this on top of them.
	ErrFailed  = errors.New("lint failed")
	errMissing = errors.New("required tool not on PATH")
)

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

	out, err := git.Output(context.Background(), args...)
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

// needTool turns a missing PATH tool into the install line for it, which is
// the only useful thing to say about that failure.
func needTool(name, install string) error {
	err := linters.Available(name)
	if err != nil {
		return fmt.Errorf("%w: %s not found. Install it with: %s", errMissing, name, install)
	}

	return nil
}

// runTool streams a gate's own output through; its exit code is the verdict.
func runTool(out io.Writer, name string, args ...string) error {
	result, err := linters.Run(context.Background(), out, name, args...)
	if err != nil || result.Code != 0 {
		return ErrFailed
	}

	return nil
}
