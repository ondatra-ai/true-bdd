package state_test

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const ticket = "86cb8hjf7"

func TestAbsentKeyIsEmpty(t *testing.T) {
	t.Parallel()

	if got := state.Get(t.TempDir(), state.TicketKey); got != "" {
		t.Fatalf("Get on a fresh checkout = %q, want none", got)
	}
}

func TestLastWriteWins(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	for _, value := range []string{"first", "second", "third"} {
		err := state.Set(repo, state.TicketKey, value)
		if err != nil {
			t.Fatalf("Set(%q): %v", value, err)
		}
	}

	if got := state.Get(repo, state.TicketKey); got != "third" {
		t.Fatalf("Get = %q, want the last value written", got)
	}
}

// Delete is Set(key, ""), and it must not disturb any other key.
func TestDeleteIsAnEmptyValue(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	for key, value := range map[string]string{state.TicketKey: ticket, state.MandateKey: ticket} {
		err := state.Set(repo, key, value)
		if err != nil {
			t.Fatalf("Set(%q): %v", key, err)
		}
	}

	err := state.Set(repo, state.MandateKey, "")
	if err != nil {
		t.Fatalf("Set(mandate, \"\"): %v", err)
	}

	if got := state.Get(repo, state.MandateKey); got != "" {
		t.Errorf("the deleted key reads %q, want none", got)
	}

	if got := state.Get(repo, state.TicketKey); got != ticket {
		t.Errorf("the other key reads %q, want it untouched", got)
	}
}

func TestInitClearsEverything(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	err := state.Set(repo, state.TicketKey, ticket)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	err = state.Init(repo)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got := state.Get(repo, state.TicketKey); got != "" {
		t.Fatalf("Get after Init = %q, want none", got)
	}

	_, err = os.Stat(state.File(repo))
	if !os.IsNotExist(err) {
		t.Fatalf("the state file survived Init (%v)", err)
	}
}

func TestInitOnNothingIsIdempotent(t *testing.T) {
	t.Parallel()

	err := state.Init(t.TempDir())
	if err != nil {
		t.Fatalf("Init with no state file: %v", err)
	}
}

// The load-bearing property: five concurrent writers is the normal case, so
// every writer's own open/write/close must land whole and last-write-wins.
func TestConcurrentSetsNeverTearOrLose(t *testing.T) {
	t.Parallel()

	const (
		writers = 8
		records = 100
	)

	repo := t.TempDir()

	var group sync.WaitGroup

	for writer := range writers {
		group.Go(func() {
			for record := range records {
				err := state.Set(repo, "writer"+strconv.Itoa(writer), strconv.Itoa(record))
				if err != nil {
					t.Errorf("Set: %v", err)

					return
				}
			}
		})
	}

	group.Wait()

	for writer := range writers {
		want := strconv.Itoa(records - 1)
		if got := state.Get(repo, "writer"+strconv.Itoa(writer)); got != want {
			t.Errorf("writer%d folds to %q, want %q", writer, got, want)
		}
	}

	assertEveryLineWhole(t, repo, writers*records)
}

func assertEveryLineWhole(t *testing.T, repo string, want int) {
	t.Helper()

	raw, err := os.ReadFile(state.File(repo))
	if err != nil {
		t.Fatalf("reading the state file: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != want {
		t.Fatalf("the file holds %d lines, want %d — a write was lost or torn", len(lines), want)
	}

	for index, line := range lines {
		var entry map[string]string
		if json.Unmarshal([]byte(line), &entry) != nil {
			t.Fatalf("line %d is torn: %q", index, line)
		}
	}
}
