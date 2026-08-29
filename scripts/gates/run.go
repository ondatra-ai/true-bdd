package gates

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/console"
)

var errGateFailed = errors.New("gate failed")

// Changed lists what this work touches against base — committed, uncommitted
// and untracked. Two-dot, NOT base...HEAD: the gates run before commit
// cuts the branch, and on main three-dot resolves to an empty diff.
func Changed(base string) ([]string, error) {
	tracked, err := gitLines("diff", "--name-only", base)
	if err != nil {
		return nil, fmt.Errorf("listing changed paths against %s: %w", base, err)
	}

	untracked, err := gitLines("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("listing untracked paths: %w", err)
	}

	return dedupe(append(tracked, untracked...)), nil
}

func gitLines(args ...string) ([]string, error) {
	out, err := exec.CommandContext(context.Background(), "git", args...).Output()
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller names which query failed.
	}

	var paths []string

	for line := range strings.SplitSeq(string(out), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}

	return paths, nil
}

func dedupe(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))

	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}

	return out
}

// Run executes each gate in order and stops at the first failure, the way
// `set -e` did when this was a list of lines in a shell script.
func Run(selected []Gate) error {
	for _, gate := range selected {
		slog.Info("Gate", "name", gate.Name, "command", strings.Join(gate.Command, " "))

		cmd := exec.CommandContext(context.Background(), gate.Command[0], gate.Command[1:]...)
		cmd.Stdout, cmd.Stderr = console.Out(), console.Err()

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("%w: %s: %w", errGateFailed, gate.Name, err)
		}
	}

	return nil
}
