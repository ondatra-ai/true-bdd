package promptprobe

import (
	"io"

	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/input"
)

// Drive runs the deterministic choice → clarify → freetext prompt sequence
// through the REAL answer collector reading from stdin (plan §4), returning the
// answers the collector recorded (in order). When TRUE_BDD_EVENTS_FILE is set
// (under `true-bdd remote`), the collector's emitter publishes the three prompt
// events the remote relays to the browser.
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
