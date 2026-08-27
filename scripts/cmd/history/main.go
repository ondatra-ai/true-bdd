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
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

var (
	errMissingTicketID = errors.New("usage: history bind <ticket-id>")
	errMissingMandate  = errors.New("usage: history mandate <ticket-id>")
)

// runMandate handles the three mandate verbs. `mandated` prints yes or no and
// still exits 0 — it answers a question rather than asserting one.
func runMandate(repo string, args []string) error {
	switch args[0] {
	case "mandate":
		const verbAndID = 2
		if len(args) < verbAndID {
			return errMissingMandate
		}

		return set(repo, state.MandateKey, args[1])
	case "unmandate":
		return set(repo, state.MandateKey, "")
	default:
		granted := state.Get(repo, state.MandateKey) != ""
		_, _ = fmt.Fprintln(os.Stdout, map[bool]string{true: "yes", false: "no"}[granted])

		return nil
	}
}

// bindTicket records the Ticket /task-start just took, so /task-done and
// /task-fail need no argument to know which one they are closing.
func bindTicket(repo string, args []string) error {
	if len(args) == 0 {
		return errMissingTicketID
	}

	id := strings.TrimSpace(args[0])
	if id == "" {
		return errMissingTicketID
	}

	return set(repo, state.TicketKey, id)
}

func set(repo, key, value string) error {
	err := state.Set(repo, key, value)
	if err != nil {
		return fmt.Errorf("writing %s: %w", key, err)
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

	repo := history.RepoRoot()
	hook := history.New(repo, role)

	switch args[0] {
	case "roll":
		return rollTask(repo)
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
		return binding(repo, args)
	}
}

// binding runs the Ticket-binding and mandate verbs the /task-* skills and
// task-handle call. An unrecognised argument is silence, as it was in Python:
// this is wired into the harness, and failing loudly fails on every prompt.
func binding(repo string, args []string) error {
	switch args[0] {
	case "mandate", "unmandate", "mandated":
		return runMandate(repo, args)
	case "bind":
		return bindTicket(repo, args[1:])
	case "bound":
		// Prints an empty line when nothing is bound: /task-done and
		// /task-fail read this and must tell "none" from a failure.
		_, _ = fmt.Fprintln(os.Stdout, state.Get(repo, state.TicketKey))

		return nil
	case "unbind":
		return set(repo, state.TicketKey, "")
	default:
		return nil
	}
}
