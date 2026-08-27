package history

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const stampLayout = "2006-01-02T15:04:05Z"

// Permissions for the history tree: a directory a person browses, files only
// this hook writes.
const (
	dirMode  = 0o755
	fileMode = 0o600
)

// appendEntry writes one heading, its stamp and its body to the Task's file.
// O_CREATE opens it on the first entry, so the five processes that share a
// Task need no agreement about which of them creates the file.
func (h *Hook) appendEntry(task, heading, body string) error {
	path := state.HistoryFile(h.repo, task)

	err := os.MkdirAll(filepath.Dir(path), dirMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	entry := fmt.Sprintf("## %s\n\n_%s · %s_\n\n%s\n\n",
		heading, time.Now().UTC().Format(stampLayout), h.gitSHA(), body)

	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	defer handle.Close() //nolint:errcheck // the write below reports the failure that matters.

	_, err = handle.WriteString(entry)
	if err != nil {
		return fmt.Errorf("appending to %s: %w", path, err)
	}

	return nil
}
