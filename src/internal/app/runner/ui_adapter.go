package runner

import (
	"bdd-cli/src/internal/app/engine"
	checklistmodels "bdd-cli/src/internal/domain/models/checklist"
	"bdd-cli/src/internal/infrastructure/input"
)

// fixLoopUI adapts the bdd-cli's UserInputCollector + display helpers
// into the engine's FixLoopUI interface. The engine drives the
// clarify/apply/refine/exit loop and calls back through this adapter
// for each user-facing step.
type fixLoopUI struct {
	collector *input.UserInputCollector
}

// NewFixLoopUI builds a FixLoopUI satisfying engine.FixLoopUI.
func NewFixLoopUI(collector *input.UserInputCollector) engine.FixLoopUI {
	return &fixLoopUI{collector: collector}
}

// AskQuestions converts engine ClarifyQuestions into the checklist
// type the collector understands, then converts the answer map back.
func (u *fixLoopUI) AskQuestions(questions []engine.ClarifyQuestion) map[string]string {
	converted := make([]checklistmodels.ClarifyQuestion, 0, len(questions))

	for _, q := range questions {
		converted = append(converted, checklistmodels.ClarifyQuestion{
			ID:       q.ID,
			Question: q.Question,
			Context:  q.Context,
			Options:  q.Options,
		})
	}

	return u.collector.AskQuestions(converted)
}

// AskApplyRefineOrExit translates the collector's string-typed
// ActionChoice into the engine's UserAction enum.
func (u *fixLoopUI) AskApplyRefineOrExit() engine.UserAction {
	switch u.collector.AskApplyRefineOrExit() {
	case input.ActionApply:
		return engine.UserActionApply
	case input.ActionRefine:
		return engine.UserActionRefine
	case input.ActionExit:
		return engine.UserActionExit
	}

	return engine.UserActionExit
}

// AskRefinementFeedback is a straight pass-through.
func (u *fixLoopUI) AskRefinementFeedback() string {
	return u.collector.AskRefinementFeedback()
}

// DisplayFixPrompt prints the fix prompt under the standard banner.
func (u *fixLoopUI) DisplayFixPrompt(prompt string) {
	displayFixPrompt(prompt)
}
