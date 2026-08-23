package promptprobe

import (
	"io"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/input"
)

// Drive runs the deterministic choice → clarify → freetext prompt
// sequence through the real stdin-backed answer collector, returning
// the answers in order; publishes prompt events when TRUE_BDD_EVENTS_FILE is set (see `true-bdd remote`).
func Drive(stdin io.Reader) []string {
	collector := input.NewUserInputCollectorFrom(stdin)

	choice := collector.AskApplyRefineOrExit()

	// Deterministic numbered options; the collector maps an answer NUMBER to
	// the option text (the same mapping the real `us` clarify flow uses).
	answers := collector.AskQuestions([]checklist.ClarifyQuestion{
		{ID: "probe-clarify", Question: "Pick an option", Options: []string{"one", "two", "three"}},
	})

	feedback := collector.AskRefinementFeedback()

	return []string{string(choice), answers["probe-clarify"], feedback}
}
