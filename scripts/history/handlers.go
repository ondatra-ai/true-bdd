package history

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// NewTask drops the state file so the next prompt opens a fresh Task. It
// takes the Ticket binding, the mandate and every cursor with it: one file
// holds all four, and all four belong to the Task that just ended.
func (h *Hook) NewTask() error {
	return state.Init(h.repo) //nolint:wrapcheck // Init already names the file it could not remove.
}

// PromptSubmit handles both hooks it is wired to: a UserPromptSubmit carries
// a prompt and logs it; a Stop carries none and logs the assistant's turn.
func (h *Hook) PromptSubmit(event Event) error {
	// Sub-agent invocations fire UserPromptSubmit too; skip them.
	if event.truthy("agent_id") {
		return nil
	}

	prompt := strings.TrimSpace(firstNonEmpty(event.str("prompt"), event.str("user_message")))

	// /task-start rolls history over; logging it would recreate the state file
	// it just deleted. Its Stop then finds no active Task and is skipped too.
	// /task-done and /task-fail are absent on purpose: they belong in the file.
	if fields := strings.Fields(prompt); len(fields) > 0 && fields[0] == "/task-start" {
		return nil
	}

	if prompt != "" {
		return h.logPrompt(event, prompt)
	}

	return h.logTurn(event)
}

// logPrompt opens a Task if none is active, then appends the prompt.
// Default role -> "## user" (a human turn); non-default (headless
// `claude -p` worker) -> "## claude to @<role>".
func (h *Hook) logPrompt(event Event, prompt string) error {
	heading := "user"
	if h.role != DefaultRole {
		heading = "claude to @" + h.role
	}

	task, err := state.Task(h.repo, event.str("session_id"), prompt)
	if err != nil {
		return fmt.Errorf("opening the task: %w", err)
	}

	return h.appendEntry(task, heading, prompt)
}

// logTurn appends the whole assistant turn — every text block since the
// last prompt. The event's final message backstops the tail in case the
// transcript hasn't flushed it yet.
func (h *Hook) logTurn(event Event) error {
	task := state.Get(h.repo, state.TaskKey)
	if task == "" {
		return nil
	}

	var blocks []string
	if path := event.str("transcript_path"); path != "" {
		blocks = turnBlocks(readJSONL(path))
	}

	if last := strings.TrimSpace(event.str("last_assistant_message")); last != "" {
		if len(blocks) == 0 || blocks[len(blocks)-1] != last {
			blocks = append(blocks, last)
		}
	}

	sessionID, promptID := event.str("session_id"), event.str("prompt_id")

	done := min(h.cursorRead(sessionID, promptID), len(blocks))
	if text := strings.Join(blocks[done:], "\n\n"); text != "" {
		err := h.appendEntry(task, h.role, text)
		if err != nil {
			return err
		}
	}

	h.cursorWrite(sessionID, promptID, len(blocks))

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
