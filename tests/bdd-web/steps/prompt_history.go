package steps

import (
	"strings"
)

// The three prompt kinds the CLI publishes, spelled once: promptKinds is the
// alternation a step pattern names one by, and these are what a clause compares.
const (
	choiceKind   = "choice"
	clarifyKind  = "clarify"
	freetextKind = "freetext"
)

// promptRecord is one prompt the scenario answered, in the order it answered
// them: the id, and the kind the dialog carried.
type promptRecord struct {
	ID   string
	Kind string
}

// notePrompt files a prompt the scenario is about to answer — the id "the same
// prompt" names, and the kind the ordering clauses read back. Re-noting the
// prompt already at the end of the history is a no-op: a poll may see one twice.
func notePrompt(state *State, label string, prompt *pendingPrompt) {
	rememberPrompt(state, label, prompt.PromptID)

	history := state.PromptHistory[label]
	if len(history) > 0 && history[len(history)-1].ID == prompt.PromptID {
		return
	}

	state.PromptHistory[label] = append(history,
		promptRecord{ID: prompt.PromptID, Kind: prompt.Kind})
}

// notePromptEvents remembers the sequence numbers the run had while it was FIRST
// blocked — the only evidence the later clauses have, since answering destroys the
// before-state. First wins: a later prompt's window already holds that growth.
func notePromptEvents(state *State, label string, detail *runDetail) {
	if detail.PendingPrompt == nil {
		return
	}

	_, snapshotted := state.PromptEvents[label]
	if snapshotted {
		return
	}

	seqs := make([]int, 0, len(detail.Events))

	for _, event := range detail.Events {
		seqs = append(seqs, event.Seq)
	}

	state.PromptEvents[label] = seqs
}

// idsOfKind is every prompt of one kind the scenario answered, in order.
func idsOfKind(history []promptRecord, kind string) []string {
	ids := make([]string, 0, len(history))

	for _, record := range history {
		if record.Kind == kind {
			ids = append(ids, record.ID)
		}
	}

	return ids
}

// promptKindList renders the prompts a run published, so a failure names what it
// actually asked rather than only the shape that was wanted.
func promptKindList(history []promptRecord) string {
	if len(history) == 0 {
		return "no prompt"
	}

	kinds := make([]string, 0, len(history))

	for _, record := range history {
		kinds = append(kinds, record.Kind)
	}

	return strings.Join(kinds, ", ")
}
