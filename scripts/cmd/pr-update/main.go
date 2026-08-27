// Command pr-update writes a pull request's title and body from the branch,
// then creates the PR or edits the existing one.
//
//	pr-update
//
// No arguments. It is the last step of scripts/commit on its own, for a branch
// that is already committed and pushed.
package main

import (
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
)

func main() {
	_, _ = fmt.Fprintln(os.Stdout, commit.Start(os.Args[1:]).UpdatePR())
}
