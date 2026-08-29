package clickup

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// ErrNoTicketBound is what every closing path reports when there is nothing to
// close — /task-done, /task-fail and task-handle alike.
var ErrNoTicketBound = errors.New("no Ticket is bound to this Task — nothing to close")

// CloseBound reads the bound Ticket, sets the status, comments why and clears
// the binding. The unbind runs LAST: a status write that failed with the
// binding gone leaves a Ticket stuck in PROCESSING that nothing closes.
func CloseBound(repo, status, comment string) (string, error) {
	ticket := state.Get(repo, state.TicketKey)
	if ticket == "" {
		return "", ErrNoTicketBound
	}

	err := Status(ticket, status, comment)
	if err != nil {
		return ticket, fmt.Errorf("closing the ticket: %w", err)
	}

	err = state.Set(repo, state.TicketKey, "")
	if err != nil {
		return ticket, fmt.Errorf("unbinding the ticket: %w", err)
	}

	slog.Info("Ticket closed and binding cleared", "ticket", ticket, "status", status)

	return ticket, nil
}
