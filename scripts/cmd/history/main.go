// Command history captures the conversation into docs/history/, and holds
// the Ticket binding that names what the current Task is working on.
//
// .claude/settings.json wires it to UserPromptSubmit and Stop with the same
// `prompt-submit` argument; the /task-* skills call `roll`, `new-task`,
// `bind`, `bound` and `unbind`. The repository is CLAUDE_PROJECT_DIR when
// Claude Code sets it, and `git rev-parse` for the `!`-injected skills that
// get no hook environment — see history.RepoRoot.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/mandate"
)

var (
	errMissingTicketID = errors.New("usage: history bind <ticket-id>")
	errMissingMandate  = errors.New("usage: history mandate <ticket-id>")
)

// runMandate handles the three mandate verbs. `mandated` prints yes or no and
// still exits 0 — it answers a question rather than asserting one.
func runMandate(args []string) error {
	repo := history.RepoRoot()

	switch args[0] {
	case "mandate":
		const verbAndID = 2
		if len(args) < verbAndID {
			return errMissingMandate
		}

		err := mandate.Grant(repo, args[1])
		if err != nil {
			return fmt.Errorf("granting the mandate: %w", err)
		}
	case "unmandate":
		err := mandate.Revoke(repo)
		if err != nil {
			return fmt.Errorf("revoking the mandate: %w", err)
		}
	default:
		_, _ = fmt.Fprintln(os.Stdout, map[bool]string{true: "yes", false: "no"}[mandate.Active(repo)])
	}

	return nil
}

// bindTicket records the Ticket /task-start just took, so /task-done and
// /task-fail need no argument to know which one they are closing.
func bindTicket(hook *history.Hook, args []string) error {
	if len(args) == 0 {
		return errMissingTicketID
	}

	err := hook.Bind(args[0])
	if err != nil {
		return fmt.Errorf("binding the ticket: %w", err)
	}

	return nil
}

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
	case "roll":
		return rollTask(hook)
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
		return binding(hook, args)
	}
}

// binding runs the Ticket-binding and mandate verbs the /task-* skills and
// task-handle call. An unrecognised argument is silence, as it was in Python:
// this is wired into the harness, and failing loudly fails on every prompt.
func binding(hook *history.Hook, args []string) error {
	switch args[0] {
	case "mandate", "unmandate", "mandated":
		return runMandate(args)
	case "bind":
		return bindTicket(hook, args[1:])
	case "bound":
		// Prints an empty line when nothing is bound: /task-done and
		// /task-fail read this and must tell "none" from a failure.
		_, _ = fmt.Fprintln(os.Stdout, hook.Bound())

		return nil
	case "unbind":
		err := hook.Unbind()
		if err != nil {
			return fmt.Errorf("unbinding the ticket: %w", err)
		}

		return nil
	default:
		return nil
	}
}
