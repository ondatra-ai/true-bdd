package alint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// scopeDir is where a run's manifest lives. CLAUDE.md keeps session-temporary
// files here, and it is gitignored, which alint does not mind: the walker's
// gitignore handling does not reach a manifest named by a rule.
const scopeDir = "tmp"

// alint resolves a manifest's entries against the manifest's OWN directory
// rather than the repository root — verified against 0.15.2, undocumented, and
// silent when wrong: the rule simply matches nothing.
const scopeEscape = ".."

// writeScope records the paths one Fix is restricted to and answers where.
// The name carries the pid because the variable IS the path: two sessions
// sharing a worktree would otherwise race on one file.
func writeScope(paths []string) (string, error) {
	manifest := filepath.Join(scopeDir, fmt.Sprintf("alint-scope-%d.txt", os.Getpid()))

	err := disk.Dir(scopeDir, disk.Private)
	if err != nil {
		return "", fmt.Errorf("preparing %s: %w", scopeDir, err)
	}

	err = disk.Write(manifest, []byte(scopeBody(paths)), disk.Private)
	if err != nil {
		return "", fmt.Errorf("writing %s: %w", manifest, err)
	}

	return manifest, nil
}

// scopeBody is the manifest alint reads: one path per line, each escaped out
// of scopeDir so it resolves against the repository root.
func scopeBody(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	lines := make([]string, 0, len(paths))

	for _, path := range paths {
		lines = append(lines, filepath.Join(scopeEscape, filepath.Clean(path)))
	}

	return strings.Join(lines, "\n") + "\n"
}
