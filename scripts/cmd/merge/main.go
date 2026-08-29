// Command merge merges a pull request: up to three review rounds, then land
// it.
//
//	merge
//
// No arguments. The PR comes from the current branch.
package main

import (
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/merge"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), "merge")

	// No slog.Error here: dief and usage logged before unwinding, and logging
	// twice would mark the report node failed twice over.
	if merge.Execute(os.Args[1:]) != nil {
		os.Exit(1)
	}
}
