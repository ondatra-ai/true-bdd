package engine

import (
	"context"
)

// CellHandler runs the per-cell state machine: Query → if fail and
// fix mode → clarification loop (ask GenFix repeatedly until it
// returns a prompt) → apply/refine/exit loop → Fix. The engine, not
// per-command code, owns this UX.
type CellHandler[I, Q any] struct {
	Query   QueryFn[I, Q]
	GenFix  GenerateFixFn[I, Q]
	Fix     FixFn[I]
	UI      FixLoopUI
	FixMode bool
}

// Handle runs one (item, query) cell.
func (h *CellHandler[I, Q]) Handle(
	ctx context.Context,
	item I,
	query Q,
) (CellResult[I], error) {
	passed, err := h.Query(ctx, item, query)
	if err != nil {
		return CellResult[I]{}, err
	}

	if passed {
		return CellResult[I]{Outcome: CellPassed, Item: item}, nil
	}

	if !h.FixMode {
		return CellResult[I]{Outcome: CellFailedNoFix, Item: item}, nil
	}

	return h.runFixLoop(ctx, item, query)
}

// runFixLoop drives the full interactive fix UX for one failed cell:
// clarification loop (≤ maxClarificationIterations) to extract an
// initial fix prompt, then apply/refine/exit loop until the user
// picks a terminal action.
func (h *CellHandler[I, Q]) runFixLoop(
	ctx context.Context,
	item I,
	query Q,
) (CellResult[I], error) {
	fixPrompt, userAnswers, err := h.runClarificationLoop(ctx, item, query)
	if err != nil {
		return CellResult[I]{}, err
	}

	if fixPrompt == "" {
		return CellResult[I]{Outcome: CellFailedNoFix, Item: item}, nil
	}

	return h.runApplyRefineExitLoop(ctx, item, query, fixPrompt, userAnswers)
}

// runClarificationLoop calls GenFix repeatedly until it returns a
// concrete fix prompt or the cap fires. Each iteration that returns
// Questions triggers AskQuestions, and the answers are folded into
// userAnswers before the next call.
func (h *CellHandler[I, Q]) runClarificationLoop(
	ctx context.Context,
	item I,
	query Q,
) (string, map[string]string, error) {
	userAnswers := make(map[string]string)

	for iteration := 1; iteration <= maxClarificationIterations; iteration++ {
		result, err := h.GenFix(ctx, item, query, userAnswers, iteration)
		if err != nil {
			return "", userAnswers, err
		}

		if result.HasFixPrompt() {
			return result.FixPrompt, userAnswers, nil
		}

		if !result.HasQuestions() {
			return "", userAnswers, nil
		}

		answers := h.UI.AskQuestions(result.Questions)

		for id, answer := range answers {
			userAnswers[id] = answer
		}
	}

	return "", userAnswers, nil
}

// runApplyRefineExitLoop displays the current fix prompt and walks
// the user through apply / refine / exit until they pick a terminal
// action or hit the refinement cap.
func (h *CellHandler[I, Q]) runApplyRefineExitLoop(
	ctx context.Context,
	item I,
	query Q,
	fixPrompt string,
	userAnswers map[string]string,
) (CellResult[I], error) {
	refinementCount := 0

	for {
		h.UI.DisplayFixPrompt(fixPrompt)

		switch h.UI.AskApplyRefineOrExit() {
		case UserActionApply:
			return h.applyFix(ctx, item, fixPrompt)

		case UserActionExit:
			return CellResult[I]{Outcome: CellUserExited, Item: item}, nil

		case UserActionRefine:
			newPrompt, refineErr := h.tryRefine(ctx, item, query, userAnswers, &refinementCount)
			if refineErr != nil {
				return CellResult[I]{}, refineErr
			}

			if newPrompt != "" {
				fixPrompt = newPrompt
			}
		}
	}
}

// applyFix runs the user-supplied Fix and wraps its return in a
// CellResult.
func (h *CellHandler[I, Q]) applyFix(
	ctx context.Context,
	item I,
	fixPrompt string,
) (CellResult[I], error) {
	newItem, err := h.Fix(ctx, item, FixDecision{
		Action:    ActionApply,
		FixPrompt: fixPrompt,
	})
	if err != nil {
		return CellResult[I]{}, err
	}

	return CellResult[I]{Outcome: CellFixed, Item: newItem}, nil
}

// tryRefine bumps the refinement counter (bounded by
// maxRefinementIterations), collects user feedback, and runs GenFix
// with the `_user_refinement` answer injected. Returns "" when the
// cap is hit, no feedback is given, or GenFix declines to produce a
// new prompt — caller keeps the existing prompt.
func (h *CellHandler[I, Q]) tryRefine(
	ctx context.Context,
	item I,
	query Q,
	userAnswers map[string]string,
	refinementCount *int,
) (string, error) {
	if *refinementCount >= maxRefinementIterations {
		return "", nil
	}

	*refinementCount++

	feedback := h.UI.AskRefinementFeedback()
	if feedback == "" {
		return "", nil
	}

	userAnswers[refinementMagicKey] = feedback

	iteration := *refinementCount + maxClarificationIterations

	result, err := h.GenFix(ctx, item, query, userAnswers, iteration)
	if err != nil {
		return "", err
	}

	if !result.HasFixPrompt() {
		return "", nil
	}

	return result.FixPrompt, nil
}
