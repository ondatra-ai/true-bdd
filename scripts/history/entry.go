package history

import (
	"fmt"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const stampLayout = "2006-01-02T15:04:05Z"

// appendEntry writes one heading, its stamp and its body to the Task's file.
// O_CREATE opens it on the first entry, so the five processes that share a
// Task need no agreement about which of them creates the file.
func (h *Hook) appendEntry(task, heading, body string) error {
	path := state.HistoryFile(h.repo, task)

	entry := fmt.Sprintf("## %s\n\n_%s · %s_\n\n%s\n",
		heading, time.Now().UTC().Format(stampLayout), h.gitSHA(), body)

	return disk.Append(path, []byte(entry), disk.Private)
}
