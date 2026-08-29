// Command pr-update writes a pull request's title and body from the branch,
// then creates the PR or edits the existing one.
//
//	pr-update
//
// No arguments. It is the last step of scripts/commit on its own, for a branch
// that is already committed and pushed.
package main

import (
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
)

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), "pr-update")

	slog.Info("Pull request updated", "result", commit.Start(os.Args[1:]).UpdatePR())
}
