// Command merge merges a pull request: up to three review rounds, then land
// it.
//
//	merge
//
// No arguments. The PR comes from the current branch.
package main

import (
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

func main() {
	merge.Start(os.Args[1:]).Main()
}
