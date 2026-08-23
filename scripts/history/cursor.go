package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// cursor records how much of the current turn is already logged. A
// blocking Stop hook can force a turn to continue; without this, the next
// Stop re-appends the whole turn instead of only the continuation.
type cursor struct {
	PromptID string `json:"prompt_id"`
	Blocks   int    `json:"blocks"`
}

func (h *Hook) cursorFile(sessionID string) string {
	name := textutil.Truncate(sessionID, sessionIDWidth)
	if name == "" {
		name = "unknown"
	}

	return filepath.Join(h.cursorDir(), name+".json")
}

// cursorRead is the number of blocks already logged for promptID, or 0 when
// the cursor belongs to a different turn or cannot be read.
func (h *Hook) cursorRead(sessionID, promptID string) int {
	if promptID == "" {
		return 0
	}

	raw, err := os.ReadFile(h.cursorFile(sessionID))
	if err != nil {
		return 0
	}

	var recorded cursor
	if json.Unmarshal(raw, &recorded) != nil || recorded.PromptID != promptID {
		return 0
	}

	return recorded.Blocks
}

// cursorWrite records the turn's progress. Best effort: losing a cursor
// duplicates a turn in the log, which is worse than the alternative only if
// it also breaks the hook, so it never does.
func (h *Hook) cursorWrite(sessionID, promptID string, blocks int) {
	if promptID == "" {
		return
	}

	err := os.MkdirAll(h.cursorDir(), dirMode)
	if err != nil {
		return
	}

	payload, err := json.Marshal(cursor{PromptID: promptID, Blocks: blocks})
	if err != nil {
		return
	}

	final := h.cursorFile(sessionID)
	temporary := fmt.Sprintf("%s.tmp.%d",
		final[:len(final)-len(filepath.Ext(final))], os.Getpid())

	if os.WriteFile(temporary, payload, fileMode) != nil {
		return
	}

	if os.Rename(temporary, final) != nil {
		_ = os.Remove(temporary)
	}
}
