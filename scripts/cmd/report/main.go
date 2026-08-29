// Command report folds the current Task's log into the tree a run walked and
// prints it: what each operation and sub-operation resulted in, and how long
// it took.
//
//	report              the newest run in the Task's log
//	report --run <id>   any run still in the file
//
// scripts/commit and scripts/merge report themselves when they finish; this is
// the same table for a run that was killed, or for one from earlier today.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

func main() {
	// No JSON sink: this command ANSWERS with a table it folded, and appending
	// that answer would make the next --run-less call render this one.
	logging.Install(logging.Stderr, "", "report")

	path := state.TaskLog(history.RepoRoot())

	err := run(path, os.Args[1:])
	if err != nil {
		slog.Error("report failed", "error", err)
		os.Exit(1)
	}
}

func run(path string, args []string) error {
	set := flag.NewFlagSet("report", flag.ContinueOnError)
	wanted := set.String("run", "", "the run to render (default: the newest in the file)")

	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing the flags: %w", err)
	}

	if *wanted == "" {
		*wanted, err = report.Latest(path)
		if err != nil {
			return fmt.Errorf("finding the newest run: %w", err)
		}
	}

	folded, err := report.Fold(path, *wanted)
	if err != nil {
		return fmt.Errorf("folding the log: %w", err)
	}

	folded.Narrate()

	return nil
}
