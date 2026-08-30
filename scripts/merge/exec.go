package merge

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// diagnosticLimit is how much of a failed operation's message a stop quotes.
const diagnosticLimit = 800

// check stops the run when an operation failed. The stop is here rather than in
// pkg/cli: whether a failure ends the run is this package's policy, and its two
// siblings answer it differently.
func (r *Run) check(what string, err error) {
	if err != nil {
		r.dief("%s: %v", what, textutil.Truncate(err.Error(), diagnosticLimit))
	}
}

// worktreeChanges is what git reports as uncommitted — empty when clean.
// Returns the raw listing for callers that print it; changedPaths answers the
// same question as a set.
func (r *Run) worktreeChanges() string {
	changes, err := git.WorktreeChanges()
	r.check("reading the worktree", err)

	return strings.TrimSpace(changes)
}

// changedPaths is the set of paths git reports as uncommitted.
func (r *Run) changedPaths() map[string]bool {
	paths, err := git.ChangedPaths()
	r.check("reading the worktree", err)

	return paths
}
