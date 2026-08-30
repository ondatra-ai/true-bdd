package taskhandle

import (
	"strconv"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
)

// gitOut runs a git command and returns its trimmed stdout. No dief: commit's
// and merge's own helpers stop the process on failure, which is the one
// behaviour this package must not inherit.
func gitOut(args ...string) (string, error) {
	return git.Output(args...)
}

// ghOut runs a gh command and returns its trimmed stdout, on the same terms.
func ghOut(args ...string) (string, error) {
	return github.Output(args...)
}

func itoa(value int) string { return strconv.Itoa(value) }
