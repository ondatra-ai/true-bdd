// Command commit runs this repository's whole pr-commit workflow: gates, the
// recording sweep, the doc-universe and memory skills, then the commit, the
// push and the pull request.
//
//	commit
//
// No arguments. Everything comes from the current checkout, and whether
// task-handle stamped a mandate decides how far the gates narrow.
package main

import (
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/commit"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), "commit")

	// No slog.Error here: dief and usage logged before unwinding, and logging
	// twice would mark the report node failed twice over.
	if commit.Execute(os.Args[1:]) != nil {
		os.Exit(1)
	}
}
