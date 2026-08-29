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
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
)

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), "commit")

	commit.Start(os.Args[1:]).Main()
}
