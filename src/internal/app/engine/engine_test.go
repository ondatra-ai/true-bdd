package engine_test

import (
	"context"
	"errors"
	"testing"

	"bdd-cli/src/internal/app/engine"
)

// Tests use small primitives so failures point at engine logic, not
// at parsing or rendering. Item type is int (interpreted as
// "version"); prompt type and query type are both string so the
// generateQ closure is trivial.

const fixMePrompt = "fix-me"

// stubUI captures and scripts all user-facing interactions the
// CellHandler drives. Each Ask method consumes one entry from the
// corresponding slice (or returns a zero value if exhausted).
type stubUI struct {
	answersScript   []map[string]string
	actionsScript   []engine.UserAction
	feedbackScript  []string
	displayedFixes  []string
	answersIdx      int
	actionsIdx      int
	feedbackIdx     int
	askQuestionsCnt int
}

func (s *stubUI) AskQuestions(_ []engine.ClarifyQuestion) map[string]string {
	s.askQuestionsCnt++

	if s.answersIdx >= len(s.answersScript) {
		return map[string]string{}
	}

	out := s.answersScript[s.answersIdx]
	s.answersIdx++

	return out
}

func (s *stubUI) AskApplyRefineOrExit() engine.UserAction {
	if s.actionsIdx >= len(s.actionsScript) {
		return engine.UserActionExit
	}

	out := s.actionsScript[s.actionsIdx]
	s.actionsIdx++

	return out
}

func (s *stubUI) AskRefinementFeedback() string {
	if s.feedbackIdx >= len(s.feedbackScript) {
		return ""
	}

	out := s.feedbackScript[s.feedbackIdx]
	s.feedbackIdx++

	return out
}

func (s *stubUI) DisplayFixPrompt(prompt string) {
	s.displayedFixes = append(s.displayedFixes, prompt)
}

func newHandler(
	t *testing.T,
	queryFn engine.QueryFn[int, string],
	genFn engine.GenerateFixFn[int, string],
	fixFn engine.FixFn[int],
	user engine.FixLoopUI,
	fixMode bool,
) *engine.CellHandler[int, string] {
	t.Helper()

	return &engine.CellHandler[int, string]{
		Query:   queryFn,
		GenFix:  genFn,
		Fix:     fixFn,
		UI:      user,
		FixMode: fixMode,
	}
}

func TestCellHandler_Passed(t *testing.T) {
	t.Parallel()

	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return true, nil },
		nil, nil, &stubUI{}, false,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellPassed {
		t.Errorf("want CellPassed, got %v", result.Outcome)
	}
}

func TestCellHandler_FailedNoFixWhenFixModeOff(t *testing.T) {
	t.Parallel()

	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		nil, nil, &stubUI{}, false,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellFailedNoFix {
		t.Errorf("want CellFailedNoFix, got %v", result.Outcome)
	}
}

func TestCellHandler_ApplyFlowFixedOnFirstIteration(t *testing.T) {
	t.Parallel()

	userIO := &stubUI{actionsScript: []engine.UserAction{engine.UserActionApply}}

	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			return engine.FixResult{FixPrompt: fixMePrompt}, nil
		},
		func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
			return item + 1, nil
		},
		userIO, true,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellFixed {
		t.Fatalf("want CellFixed, got %v", result.Outcome)
	}

	if result.Item != 2 {
		t.Errorf("want item=2 after fix, got %d", result.Item)
	}

	if len(userIO.displayedFixes) != 1 {
		t.Errorf("want 1 fix display, got %d", len(userIO.displayedFixes))
	}
}

func TestCellHandler_ClarificationLoopThenApply(t *testing.T) {
	t.Parallel()

	userIO := &stubUI{
		answersScript: []map[string]string{{"q1": "yes"}},
		actionsScript: []engine.UserAction{engine.UserActionApply},
	}

	calls := 0
	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, answers map[string]string, _ int) (engine.FixResult, error) {
			calls++
			if calls == 1 {
				return engine.FixResult{Questions: []engine.ClarifyQuestion{{ID: "q1", Question: "Why?"}}}, nil
			}

			if answers["q1"] != "yes" {
				t.Errorf("want answer to flow through, got %q", answers["q1"])
			}

			return engine.FixResult{FixPrompt: fixMePrompt}, nil
		},
		func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
			return item + 1, nil
		},
		userIO, true,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellFixed {
		t.Errorf("want CellFixed, got %v", result.Outcome)
	}

	if calls != 2 {
		t.Errorf("want 2 GenFix calls (questions then prompt), got %d", calls)
	}
}

func TestCellHandler_UserExited(t *testing.T) {
	t.Parallel()

	userIO := &stubUI{actionsScript: []engine.UserAction{engine.UserActionExit}}

	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			return engine.FixResult{FixPrompt: fixMePrompt}, nil
		},
		nil, userIO, true,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellUserExited {
		t.Errorf("want CellUserExited, got %v", result.Outcome)
	}
}

func TestCellHandler_ClarificationCapTreatsAsFailedNoFix(t *testing.T) {
	t.Parallel()

	// GenFix always returns Questions, never a FixPrompt — engine
	// should bail after the clarification cap.
	userIO := &stubUI{answersScript: []map[string]string{
		{"q1": "a"}, {"q1": "b"}, {"q1": "c"}, {"q1": "d"}, {"q1": "e"},
	}}

	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			return engine.FixResult{Questions: []engine.ClarifyQuestion{{ID: "q1"}}}, nil
		},
		nil, userIO, true,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellFailedNoFix {
		t.Errorf("want CellFailedNoFix after cap, got %v", result.Outcome)
	}
}

func TestCellHandler_RefinePathInjectsFeedback(t *testing.T) {
	t.Parallel()

	userIO := &stubUI{
		actionsScript:  []engine.UserAction{engine.UserActionRefine, engine.UserActionApply},
		feedbackScript: []string{"please be more specific"},
	}

	genCalls := 0
	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, answers map[string]string, iteration int) (engine.FixResult, error) {
			genCalls++

			if genCalls == 2 {
				// Refinement iteration: answer slot must carry the feedback
				// and iteration must be in the refinement range.
				if answers["_user_refinement"] != "please be more specific" {
					t.Errorf("want refinement feedback in answers, got %q", answers["_user_refinement"])
				}

				if iteration <= 5 {
					t.Errorf("want iteration in refinement range, got %d", iteration)
				}

				return engine.FixResult{FixPrompt: "refined-fix"}, nil
			}

			return engine.FixResult{FixPrompt: "initial-fix"}, nil
		},
		func(_ context.Context, item int, decision engine.FixDecision) (int, error) {
			if decision.FixPrompt != "refined-fix" {
				t.Errorf("want refined-fix prompt applied, got %q", decision.FixPrompt)
			}

			return item + 1, nil
		},
		userIO, true,
	)

	result, err := handler.Handle(context.Background(), 1, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outcome != engine.CellFixed {
		t.Errorf("want CellFixed, got %v", result.Outcome)
	}
}

func TestCellHandler_PropagatesQueryError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("query boom")
	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, sentinel },
		nil, nil, &stubUI{}, true,
	)

	_, err := handler.Handle(context.Background(), 1, "q")
	if !errors.Is(err, sentinel) {
		t.Errorf("want sentinel, got %v", err)
	}
}

func TestCellHandler_PropagatesGenFixError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("genfix boom")
	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			return engine.FixResult{}, sentinel
		},
		nil, &stubUI{}, true,
	)

	_, err := handler.Handle(context.Background(), 1, "q")
	if !errors.Is(err, sentinel) {
		t.Errorf("want sentinel, got %v", err)
	}
}

func TestCellHandler_PropagatesFixError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("fix boom")
	userIO := &stubUI{actionsScript: []engine.UserAction{engine.UserActionApply}}

	handler := newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return false, nil },
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			return engine.FixResult{FixPrompt: "fp"}, nil
		},
		func(_ context.Context, _ int, _ engine.FixDecision) (int, error) {
			return 0, sentinel
		},
		userIO, true,
	)

	_, err := handler.Handle(context.Background(), 1, "q")
	if !errors.Is(err, sentinel) {
		t.Errorf("want sentinel, got %v", err)
	}
}

// --- SequentialWalker ---

// passingHandler builds a CellHandler whose Query always passes — used
// to exercise walker iteration alone.
func passingHandler(t *testing.T) *engine.CellHandler[int, string] {
	t.Helper()

	return newHandler(t,
		func(_ context.Context, _ int, _ string) (bool, error) { return true, nil },
		nil, nil, &stubUI{}, false,
	)
}

func TestSequentialWalker_AllPass(t *testing.T) {
	t.Parallel()

	walker := &engine.SequentialWalker[int, string]{Cell: passingHandler(t)}

	out, err := walker.Walk(context.Background(), 1, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !out.Passed || out.FixApplied || out.UserExited {
		t.Errorf("want passed/no-fix/no-exit, got %+v", out)
	}
}

func TestSequentialWalker_FailNonFixModeContinues(t *testing.T) {
	t.Parallel()

	calls := 0
	handler := newHandler(t,
		func(_ context.Context, _ int, query string) (bool, error) {
			calls++

			return query != "b", nil
		},
		nil, nil, &stubUI{}, false,
	)
	walker := &engine.SequentialWalker[int, string]{Cell: handler}

	out, err := walker.Walk(context.Background(), 1, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Passed {
		t.Errorf("want passed=false")
	}

	if calls != 3 {
		t.Errorf("want 3 query calls, got %d", calls)
	}
}

func TestSequentialWalker_FixRestartsFromZero(t *testing.T) {
	t.Parallel()

	// Pass when item >= 2; fail when item < 2. Fix increments item.
	// Expect: idx=0 fail → fix → restart → all pass.
	userIO := &stubUI{actionsScript: []engine.UserAction{engine.UserActionApply}}

	queryCalls := 0
	fixCalls := 0
	handler := newHandler(t,
		func(_ context.Context, item int, _ string) (bool, error) {
			queryCalls++

			return item >= 2, nil
		},
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			return engine.FixResult{FixPrompt: "x"}, nil
		},
		func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
			fixCalls++

			return item + 1, nil
		},
		userIO, true,
	)
	walker := &engine.SequentialWalker[int, string]{Cell: handler}

	out, err := walker.Walk(context.Background(), 1, []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !out.FixApplied {
		t.Errorf("want FixApplied=true")
	}

	if !out.Passed {
		t.Errorf("want passed=true after fix, got %+v", out)
	}

	if out.Item != 2 {
		t.Errorf("want item=2 after fix, got %d", out.Item)
	}

	if fixCalls != 1 {
		t.Errorf("want 1 fix call, got %d", fixCalls)
	}

	if queryCalls != 3 { // first fail, then restart: pass, pass.
		t.Errorf("want 3 query calls, got %d", queryCalls)
	}
}

// --- Engine ---

// stubWalker returns canned ItemRuns in order.
type stubWalker[I, Q any] struct {
	runs  []engine.ItemRun[I]
	calls int
}

func (s *stubWalker[I, Q]) Walk(_ context.Context, _ I, _ []Q) (engine.ItemRun[I], error) {
	run := s.runs[s.calls%len(s.runs)]
	s.calls++

	return run, nil
}

func TestEngine_EmptyPromptsConverges(t *testing.T) {
	t.Parallel()

	eng := engine.New[int, string, string](
		func(_ int, prompt string) string { return prompt },
		&stubWalker[int, string]{},
		engine.Options{},
	)

	result, err := eng.Run(context.Background(), []int{1, 2, 3}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Reason != engine.Converged || !result.AllPassed {
		t.Errorf("want Converged+AllPassed, got %+v", result)
	}
}

func TestEngine_SingleWalkConverges(t *testing.T) {
	t.Parallel()

	walker := &stubWalker[int, string]{runs: []engine.ItemRun[int]{
		{Item: 1, Passed: true},
	}}
	eng := engine.New[int, string, string](
		func(_ int, prompt string) string { return prompt },
		walker,
		engine.Options{},
	)

	result, err := eng.Run(context.Background(), []int{1}, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Reason != engine.Converged || !result.AllPassed {
		t.Errorf("want Converged+AllPassed, got %+v", result)
	}

	if walker.calls != 1 {
		t.Errorf("want 1 walker call, got %d", walker.calls)
	}
}

func TestEngine_FixThenRewalkConverges(t *testing.T) {
	t.Parallel()

	// First walk: one item that applied a fix; second walk: same item
	// passes with no fix → converged.
	walker := &stubWalker[int, string]{runs: []engine.ItemRun[int]{
		{Item: 1, Passed: true, FixApplied: true},
		{Item: 1, Passed: true, FixApplied: false},
	}}
	eng := engine.New[int, string, string](
		func(_ int, prompt string) string { return prompt },
		walker,
		engine.Options{MaxApplyAttempts: 5},
	)

	result, err := eng.Run(context.Background(), []int{1}, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Reason != engine.Converged {
		t.Errorf("want Converged, got %v", result.Reason)
	}

	if walker.calls != 2 {
		t.Errorf("want 2 walker calls, got %d", walker.calls)
	}
}

func TestEngine_NotFixedWhenNoFixApplied(t *testing.T) {
	t.Parallel()

	walker := &stubWalker[int, string]{runs: []engine.ItemRun[int]{
		{Item: 1, Passed: false, FixApplied: false},
	}}
	eng := engine.New[int, string, string](
		func(_ int, prompt string) string { return prompt },
		walker,
		engine.Options{},
	)

	result, err := eng.Run(context.Background(), []int{1}, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Reason != engine.NotFixed || result.AllPassed {
		t.Errorf("want NotFixed+!AllPassed, got %+v", result)
	}
}

func TestEngine_UserExitShortCircuits(t *testing.T) {
	t.Parallel()

	walker := &stubWalker[int, string]{runs: []engine.ItemRun[int]{
		{Item: 1, Passed: false, UserExited: true},
	}}
	eng := engine.New[int, string, string](
		func(_ int, prompt string) string { return prompt },
		walker,
		engine.Options{},
	)

	result, err := eng.Run(context.Background(), []int{1, 2, 3}, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Reason != engine.UserExit {
		t.Errorf("want UserExit, got %v", result.Reason)
	}

	if walker.calls != 1 {
		t.Errorf("want 1 walker call after exit, got %d", walker.calls)
	}
}

func TestEngine_MaxAttemptsExhausted(t *testing.T) {
	t.Parallel()

	walker := &stubWalker[int, string]{runs: []engine.ItemRun[int]{
		{Item: 1, Passed: true, FixApplied: true},
	}}
	eng := engine.New[int, string, string](
		func(_ int, prompt string) string { return prompt },
		walker,
		engine.Options{MaxApplyAttempts: 3},
	)

	result, err := eng.Run(context.Background(), []int{1}, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Reason != engine.MaxAttemptsExhausted {
		t.Errorf("want MaxAttemptsExhausted, got %v", result.Reason)
	}

	if walker.calls != 3 {
		t.Errorf("want 3 walker calls, got %d", walker.calls)
	}
}

func TestEngine_GenerateQReceivesOneBasedIndex(t *testing.T) {
	t.Parallel()

	indices := []int{}
	walker := &stubWalker[int, string]{runs: []engine.ItemRun[int]{
		{Item: 1, Passed: true},
	}}
	eng := engine.New[int, string, string](
		func(idx int, prompt string) string {
			indices = append(indices, idx)

			return prompt
		},
		walker,
		engine.Options{},
	)

	_, err := eng.Run(context.Background(), []int{1}, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(indices) != 3 || indices[0] != 1 || indices[1] != 2 || indices[2] != 3 {
		t.Errorf("want 1-based indices [1,2,3], got %v", indices)
	}
}
