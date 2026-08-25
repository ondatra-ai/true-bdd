package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errEmptyTicketID = errors.New("a ticket id is required")

// bindingFile holds one line: the ClickUp Ticket this Task is working on.
// Beside hook-state on purpose — same directory, same lifecycle, same
// question — and docs/history/ is gitignored, so it never reaches a commit.
func (h *Hook) bindingFile() string {
	return filepath.Join(h.historyDir(), "bound-ticket")
}

// Bind records the Ticket this Task is working on. Its span is
// [/task-start, /task-done|/task-fail], which is PROCESSING exactly.
func (h *Hook) Bind(ticketID string) error {
	id := strings.TrimSpace(ticketID)
	if id == "" {
		return errEmptyTicketID
	}

	return h.writeAtomic(h.bindingFile(), id)
}

// Bound reports the bound Ticket, or "" when there is none. A read failure
// reads as "none", the way loadCurrent treats one: recovery is the same.
func (h *Hook) Bound() string {
	raw, err := os.ReadFile(h.bindingFile())
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(raw))
}

// Unbind drops the binding. Called only after the status write it closes has
// succeeded, so a failed /task-done leaves the Ticket bound and retryable.
func (h *Hook) Unbind() error {
	err := os.Remove(h.bindingFile())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", h.bindingFile(), err)
	}

	return nil
}
