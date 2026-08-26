package mandate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/mandate"
)

const ticket = "86cb8hjf7"

func TestNoMandateByDefault(t *testing.T) {
	t.Parallel()

	if mandate.Active(t.TempDir()) {
		t.Fatal("a fresh checkout reports a mandate")
	}
}

func TestActiveOnlyWhileItsTicketIsBound(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	hook := history.New(repo, history.DefaultRole)

	err := mandate.Grant(repo, ticket)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	if mandate.Active(repo) {
		t.Error("a mandate with nothing bound is honoured; it must not be")
	}

	err = hook.Bind(ticket)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}

	if !mandate.Active(repo) {
		t.Error("a mandate naming the bound Ticket is not honoured")
	}

	// This is what /task-done does, and it is what makes a mandate left by a
	// dead session unable to authorise anything later.
	err = hook.Unbind()
	if err != nil {
		t.Fatalf("Unbind: %v", err)
	}

	if mandate.Active(repo) {
		t.Error("the mandate outlived its binding")
	}
}

func TestStaleMandateNamesAnotherTicket(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	err := mandate.Grant(repo, "86cbOLD1")
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	err = history.New(repo, history.DefaultRole).Bind(ticket)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}

	if mandate.Active(repo) {
		t.Error("a mandate for a different Ticket is honoured")
	}
}

func TestRevoke(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	err := mandate.Revoke(repo)
	if err != nil {
		t.Fatalf("Revoke with no mandate: %v", err)
	}

	err = mandate.Grant(repo, ticket)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	err = mandate.Revoke(repo)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = os.Stat(mandate.File(repo))
	if !os.IsNotExist(err) {
		t.Fatalf("%s survived Revoke", mandate.File(repo))
	}
}

// It must NOT be under docs/history/, which /task-start clears per Ticket.
func TestMandateLivesOutsideTheHistoryTree(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	want := filepath.Join(repo, "tmp", "history-cursor", "mandate.json")

	if got := mandate.File(repo); got != want {
		t.Fatalf("File() = %q, want %q", got, want)
	}
}
