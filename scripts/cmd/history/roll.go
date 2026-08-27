package main

import (
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// rollTask rolls the Task state and nothing else: the state file goes, so the
// next prompt opens a fresh Task under docs/history/. It does NOT touch the
// working tree — starting a Task must not discard uncommitted work.
func rollTask(repo string) error {
	// Whatever is bound belongs to the Task that just ended. Init drops it
	// with everything else — but say which one went: it is still PROCESSING
	// and nothing else will close it.
	orphan := state.Get(repo, state.TicketKey)

	err := state.Init(repo)
	if err != nil {
		return fmt.Errorf("rolling the task state: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout,
		"history rolled: the next prompt opens a fresh file in docs/history/")

	if orphan != "" {
		_, _ = fmt.Fprintf(os.Stdout,
			"WARNING: ticket %s was still bound and is now unbound.\n"+
				"It is still PROCESSING in ClickUp — nobody closed it. Tell the user.\n", orphan)
	}

	return nil
}
