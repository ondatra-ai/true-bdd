// Package mandate records that a run is being driven by handle-loop, so a
// separate process — scripts/merge — can tell an unattended merge from a
// human one and apply the right triage floors.
//
// Two placement facts, both learned the hard way and both load-bearing.
//
// It is NOT in docs/history/. /task-start deletes hook-state and clears the
// binding once per Ticket, and a mandate spans the whole run: stored there it
// would die at exactly the boundary it has to survive. tmp/history-cursor/
// outlives /task-start by construction and nothing prunes it.
//
// Because nothing prunes it, a mandate left by a dead session would otherwise
// sit there authorising merges forever. So it carries the Ticket it was
// stamped for, and Active honours it only while that same Ticket is still
// bound — which is the exact window handle-loop merges in. handle-loop
// re-stamps at every Ticket; a stale file can never match.
package mandate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/scripts/history"
)

// Permissions match the cursor's, which shares this directory.
const (
	dirMode  = 0o755
	fileMode = 0o600
)

type record struct {
	Ticket string `json:"ticket"`
}

// File is where the mandate lives, beside the per-session turn cursor.
func File(repo string) string {
	return filepath.Join(repo, "tmp", "history-cursor", "mandate.json")
}

// Grant stamps the mandate for the Ticket about to be worked.
func Grant(repo, ticketID string) error {
	err := os.MkdirAll(filepath.Dir(File(repo)), dirMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(File(repo)), err)
	}

	payload, err := json.Marshal(record{Ticket: ticketID})
	if err != nil {
		return fmt.Errorf("encoding the mandate: %w", err)
	}

	err = os.WriteFile(File(repo), payload, fileMode)
	if err != nil {
		return fmt.Errorf("writing %s: %w", File(repo), err)
	}

	return nil
}

// Revoke drops the mandate. Cancellation must be an explicit write: nothing
// here expires on its own.
func Revoke(repo string) error {
	err := os.Remove(File(repo))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", File(repo), err)
	}

	return nil
}

// Active reports whether this merge is running unattended: a mandate exists
// and names the Ticket that is bound right now.
func Active(repo string) bool {
	raw, err := os.ReadFile(File(repo))
	if err != nil {
		return false
	}

	var stamped record
	if json.Unmarshal(raw, &stamped) != nil || stamped.Ticket == "" {
		return false
	}

	return stamped.Ticket == history.New(repo, history.DefaultRole).Bound()
}
