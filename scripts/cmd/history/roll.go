package main

import (
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/history"
)

// rollTask rolls the Task history state and nothing else: the state file goes,
// so the next prompt opens a fresh file under docs/history/. It does NOT touch
// the working tree — starting a Task must not discard uncommitted work.
func rollTask(hook *history.Hook) error {
	// Whatever is bound belongs to the Task that just ended. Leaving it would
	// let a later /task-done close a Ticket this Task never touched — but say
	// which one was dropped: it is still PROCESSING and nothing else will.
	orphan := hook.Bound()

	err := hook.NewTask()
	if err != nil {
		return fmt.Errorf("rolling the task history: %w", err)
	}

	err = hook.Unbind()
	if err != nil {
		return fmt.Errorf("unbinding the ticket: %w", err)
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
