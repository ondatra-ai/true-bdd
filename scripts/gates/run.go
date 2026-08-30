package gates

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

var errGateFailed = errors.New("gate failed")

// Changed lists what this work touches against base — committed, uncommitted
// and untracked.
func Changed(base string) ([]string, error) {
	tracked, err := git.ChangedAgainst(base)
	if err != nil {
		return nil, fmt.Errorf("listing changed paths against %s: %w", base, err)
	}

	untracked, err := git.UntrackedPaths()
	if err != nil {
		return nil, fmt.Errorf("listing untracked paths: %w", err)
	}

	return dedupe(append(tracked, untracked...)), nil
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
		command := strings.Join(gate.Command, " ")
		slog.Info("Gate", "name", gate.Name, "command", command)

		started := time.Now()

		result, err := spec.Run(gate.Command,
			cli.Options{Output: cli.Console()})
		if err == nil {
			err = result.Err()
		}

		if err != nil {
			// Reported before the return: a red gate is the one whose duration
			// a reader most wants, and the stop above would swallow it.
			report.Leaf(gate.Name, started, "command", command, report.KeyStatus, report.StatusFailed)

			return fmt.Errorf("%w: %s: %w", errGateFailed, gate.Name, err)
		}

		report.Leaf(gate.Name, started, "command", command)
	}

	return nil
}
