// Command history captures the conversation into docs/history/.
//
// Invoked by .claude/hooks/history.sh, which is wired to UserPromptSubmit and
// Stop with the same `prompt-submit` argument, and by .claude/commands/
// new-task.sh with `new-task`.
package main

import (
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/history"
)

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	role := history.Role()
	if role == history.OffRole {
		return nil
	}

	if len(args) == 0 {
		return nil
	}

	hook := history.New(history.RepoRoot(), role)

	switch args[0] {
	case "new-task":
		// Never touches stdin. The `!`-invoked slash command may inherit an
		// interactive one, and a read would block forever, hanging the
		// command so the state file is never deleted.
		err := hook.NewTask()
		if err != nil {
			return fmt.Errorf("rolling the task history: %w", err)
		}

		return nil
	case "prompt-submit":
		err := hook.PromptSubmit(history.DecodeEvent(os.Stdin))
		if err != nil {
			return fmt.Errorf("capturing the turn: %w", err)
		}

		return nil
	default:
		// An unrecognised argument is silence, as it was in Python: this is
		// wired into the harness, and a hook that fails loudly on a
		// mis-wiring fails on every prompt.
		return nil
	}
}
