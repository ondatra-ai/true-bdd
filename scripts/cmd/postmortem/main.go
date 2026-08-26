// Command postmortem reads one merge run back and files what it suggests.
//
//	postmortem --pr 99 --history docs/history/<file>.md [--timings tmp/merge/timings.json]
//
// The merge loop runs this at its own tail, but only for a run that earned it,
// so this is how a postmortem is asked for deliberately. The history file is
// an argument because docs/history/hook-state is one repo-global pointer: with
// two live sessions in one checkout, PR #99's postmortem read the other one's.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/merge"
)

var errUsage = errors.New(
	"usage: postmortem --pr <n> --history <path> [--timings <path>]")

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	set := flag.NewFlagSet("postmortem", flag.ContinueOnError)
	number := set.Int("pr", 0, "the pull request the proposals are filed against")
	history := set.String("history", "", "the session history file to read the run back from")
	timings := set.String("timings", merge.StateDir+"/timings.json",
		"the run's timing record, as written by the merge loop")

	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if *number == 0 || *history == "" {
		return errUsage
	}

	err = enterRepositoryRoot()
	if err != nil {
		return err
	}

	merge.Postmortem(merge.PostmortemOptions{
		PR:          *number,
		HistoryFile: *history,
		Timings:     renderTimings(*timings),
		Floor:       merge.PostmortemFloorNow(),
	})

	return nil
}

// enterRepositoryRoot puts every relative path — the history file, the state
// directory, the worktree check — on the same footing the merge loop uses.
func enterRepositoryRoot() error {
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output() //nolint:noctx // one git read.
	if err != nil {
		return fmt.Errorf("not inside a git repository: %w", err)
	}

	err = os.Chdir(strings.TrimSpace(string(top)))
	if err != nil {
		return fmt.Errorf("cannot enter the repository root: %w", err)
	}

	return nil
}

// renderTimings turns the merge loop's record back into the table the prompt
// carries. An absent record is not a failure: a run that never got that far is
// exactly the kind worth reading back.
func renderTimings(path string) string {
	raw, err := os.ReadFile(path) //nolint:gosec // the caller names the file it wants read.
	if err != nil {
		return ""
	}

	var record merge.TimingReport

	err = json.Unmarshal(raw, &record)
	if err != nil {
		return ""
	}

	return record.Render()
}
