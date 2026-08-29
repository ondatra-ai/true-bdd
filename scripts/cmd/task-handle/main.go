// Command task-handle takes one ClickUp Ticket from TO DO to merged and DONE,
// unattended: check, start, work, scope-check, commit, review, merge, close,
// report, log.
//
//	task-handle <ticket-id>
//
// It exits 0 whenever it produced a verdict — DONE, FAILED, halted, awaiting
// merge and not started are all verdicts, and task-loop continues on every
// one. A non-zero exit means the protocol itself could not run.
package main

import (
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), "task-handle")

	run, err := taskhandle.Start(os.Args[1:])
	if err != nil {
		slog.Error("task-handle could not start", "error", err)
		os.Exit(1)
	}

	taskhandle.Main(run)
}
