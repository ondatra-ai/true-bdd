// Command merge merges a pull request: up to three review rounds, then land
// it.
//
//	merge
//
// No arguments. The PR comes from the current branch.
package main

import (
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), "merge")

	merge.Start(os.Args[1:]).Main()
}
