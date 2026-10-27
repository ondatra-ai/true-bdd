package runner

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	checklistmodels "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
)

// fixLoopUI adapts true-bdd's UserInputCollector + display helpers into
// the engine's FixLoopUI interface: the engine drives the
// clarify/apply/refine/exit loop and calls back through this adapter.
type fixLoopUI struct {
	collector *input.UserInputCollector
}

// NewFixLoopUI builds a FixLoopUI satisfying engine.FixLoopUI.
func NewFixLoopUI(collector *input.UserInputCollector) engine.FixLoopUI {
	return &fixLoopUI{collector: collector}
}

// AskQuestions converts engine ClarifyQuestions into the checklist
// type the collector understands, then converts the answer map back.
func (u *fixLoopUI) AskQuestions(questions []engine.ClarifyQuestion) (map[string]string, error) {
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
func (u *fixLoopUI) AskApplyRefineOrExit() (engine.UserAction, error) {
	action, err := u.collector.AskApplyRefineOrExit()
	if err != nil {
		return engine.UserActionExit, err
	}

	switch action {
	case input.ActionApply:
		return engine.UserActionApply, nil
	case input.ActionRefine:
		return engine.UserActionRefine, nil
	case input.ActionExit:
		return engine.UserActionExit, nil
	}

	return engine.UserActionExit, nil
}

// AskRefinementFeedback is a straight pass-through.
func (u *fixLoopUI) AskRefinementFeedback() (string, error) {
	return u.collector.AskRefinementFeedback()
}

// DisplayFixPrompt prints the fix prompt under the standard banner.
func (u *fixLoopUI) DisplayFixPrompt(prompt string) {
	displayFixPrompt(prompt)
}
