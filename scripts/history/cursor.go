package history

import (
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// The cursor records how much of the current turn is already logged, as
// "<prompt-id>:<blocks>". A blocking Stop hook can force a turn to continue;
// without this, the next Stop re-appends the whole turn.
func (h *Hook) cursorRead(sessionID, promptID string) int {
	if promptID == "" {
		return 0
	}

	turn, blocks, found := strings.Cut(state.Get(h.repo, state.CursorKey(sessionID)), ":")
	if !found || turn != promptID {
		return 0
	}

	count, err := strconv.Atoi(blocks)
	if err != nil {
		return 0
	}

	return count
}

// cursorWrite records the turn's progress. Best effort: losing a cursor
// duplicates a turn in the history, which never justifies failing the hook.
func (h *Hook) cursorWrite(sessionID, promptID string, blocks int) {
	if promptID == "" {
		return
	}

	_ = state.Set(h.repo, state.CursorKey(sessionID), promptID+":"+strconv.Itoa(blocks))
}
