package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/history"
)

// runClose is the whole terminal transition /task-done and /task-fail need.
// Order is the point: the unbind runs LAST, because a status write that failed
// with the binding gone leaves a Ticket stuck in PROCESSING that nothing closes.
func runClose(args []string) error {
	const wanted = 2
	if len(args) < wanted {
		return fmt.Errorf("%w\n%s", errMissingFlag, usage)
	}

	hook := history.New(history.RepoRoot(), history.Role())

	ticket := hook.Bound()
	if ticket == "" {
		return errNoTicketBound
	}

	err := clickup.Status(os.Stdout, ticket, args[0], strings.Join(args[1:], " "))
	if err != nil {
		return fmt.Errorf("closing the ticket: %w", err)
	}

	err = hook.Unbind()
	if err != nil {
		return fmt.Errorf("unbinding the ticket: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "closed %s as %s; binding cleared\n", ticket, args[0])

	return nil
}
