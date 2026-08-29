package main

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/history"
)

// runClose is what /task-done and /task-fail shell out to. The transition
// itself lives in scripts/clickup, so task-handle reaches it as a package
// import rather than through a second copy of the ordering.
func runClose(args []string) error {
	const wanted = 2
	if len(args) < wanted {
		return fmt.Errorf("%w\n%s", errMissingFlag, usage)
	}

	_, err := clickup.CloseBound(history.RepoRoot(), args[0], strings.Join(args[1:], " "))
	if err != nil {
		return fmt.Errorf("closing the bound Ticket: %w", err)
	}

	return nil
}
