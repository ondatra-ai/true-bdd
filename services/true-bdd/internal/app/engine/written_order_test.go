package engine_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
)

const (
	wordQuestion    = "words"
	applyEvent      = "apply"
	linkQuestion    = "link"
	codeQuestion    = "code"
	earlierQuestion = "earlier"
	orderFix        = "repair"
	laterQuestion   = "later"
)

func TestWalkersRecheckTheFailedQuestionBeforeRestarting(t *testing.T) {
	t.Parallel()

	for _, concurrent := range []bool{false, true} {
		t.Run(walkerName(concurrent), func(t *testing.T) {
			t.Parallel()

			var calls []string

			handler := newHandler(t,
				func(_ context.Context, version int, query string) (bool, error) {
					calls = append(calls, query)

					return query != codeQuestion || version >= 2, nil
				},
				func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
					return engine.FixResult{FixPrompt: orderFix}, nil
				},
				func(_ context.Context, version int, _ engine.FixDecision) (int, error) {
					calls = append(calls, applyEvent)

					return version + 1, nil
				},
				&stubUI{actionsScript: []engine.UserAction{engine.UserActionApply, engine.UserActionApply}}, true,
			)
			walker := orderWalker(concurrent, handler)

			run, err := walker.Walk(t.Context(), 0, []string{earlierQuestion, codeQuestion, laterQuestion})
			if err != nil {
				t.Fatal(err)
			}

			want := []string{earlierQuestion, codeQuestion, applyEvent, codeQuestion, applyEvent,
				codeQuestion, earlierQuestion, codeQuestion, laterQuestion}
			if !reflect.DeepEqual(calls, want) || !run.Passed || !run.FixApplied {
				t.Fatalf("calls=%v run=%+v; want %v", calls, run, want)
			}
		})
	}
}

func TestWalkersReportEveryFailureWithoutFix(t *testing.T) {
	t.Parallel()

	for _, concurrent := range []bool{false, true} {
		t.Run(walkerName(concurrent), func(t *testing.T) {
			t.Parallel()

			var calls []string

			handler := newHandler(t,
				func(_ context.Context, _ int, query string) (bool, error) {
					calls = append(calls, query)

					return false, nil
				}, nil, nil, &stubUI{}, false,
			)

			run, err := orderWalker(concurrent, handler).Walk(t.Context(), 0, []string{"prompt", codeQuestion, laterQuestion})
			if err != nil {
				t.Fatal(err)
			}

			if run.Passed || !reflect.DeepEqual(calls, []string{"prompt", codeQuestion, laterQuestion}) {
				t.Fatalf("calls=%v run=%+v", calls, run)
			}
		})
	}
}

func walkerName(concurrent bool) string {
	if concurrent {
		return "concurrent"
	}

	return "sequential"
}

func orderWalker(concurrent bool, handler *engine.CellHandler[int, string]) engine.Walker[int, string] {
	if concurrent {
		return &engine.ConcurrentWalker[int, string]{Limit: 2,
			CellFor: func(int, string) *engine.CellHandler[int, string] { return handler },
		}
	}

	return &engine.SequentialWalker[int, string]{Cell: handler}
}

func TestConcurrentWalkerInvalidatesBeforeRecheckingSharedBindings(t *testing.T) {
	t.Parallel()

	const bindingQuestion = "bindings"

	var (
		calls  []string
		cached *bool
	)

	repaired := false
	walker := &engine.ConcurrentWalker[int, string]{Limit: 2,
		OnValidate:    func([]int) { cached = nil },
		BeforeRecheck: func() { calls = append(calls, "invalidate"); cached = nil },
	}
	walker.CellFor = func(_ int, _ string) *engine.CellHandler[int, string] {
		return newHandler(t,
			func(_ context.Context, _ int, query string) (bool, error) {
				calls = append(calls, query)
				if query != bindingQuestion {
					return true, nil
				}

				if cached == nil {
					value := repaired
					cached = &value
				}

				return *cached, nil
			},
			func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
				return engine.FixResult{FixPrompt: orderFix}, nil
			},
			func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
				calls = append(calls, applyEvent)
				repaired = true

				return item, nil
			}, &stubUI{actionsScript: []engine.UserAction{engine.UserActionApply}}, true,
		)
	}

	run, err := walker.Walk(t.Context(), 0, []string{linkQuestion, bindingQuestion, wordQuestion})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{linkQuestion, bindingQuestion, applyEvent, "invalidate",
		bindingQuestion, linkQuestion, bindingQuestion, wordQuestion}
	if !run.Passed || !reflect.DeepEqual(calls, want) {
		t.Fatalf("run=%+v calls=%v; want %v", run, calls, want)
	}
}
