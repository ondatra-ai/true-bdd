package engine_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestWalkersValidateEveryAllowedApplyBeforeStopping(t *testing.T) {
	t.Parallel()

	for _, concurrent := range []bool{false, true} {
		for _, converges := range []bool{false, true} {
			name := walkerName(concurrent) + map[bool]string{false: "/exhausted", true: "/last-apply-passes"}[converges]

			t.Run(name, func(t *testing.T) {
				t.Parallel()
				assertApplyBound(t, concurrent, converges)
			})
		}
	}
}

func assertApplyBound(t *testing.T, concurrent, converges bool) {
	t.Helper()

	var events []string

	handler := newHandler(t,
		func(_ context.Context, item int, _ string) (bool, error) {
			events = append(events, "Q")

			return converges && item == 2, nil
		},
		func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
			events = append(events, "F")

			return engine.FixResult{FixPrompt: "repair"}, nil
		},
		func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
			events = append(events, "apply")

			return item + 1, nil
		}, &stubUI{actionsScript: []engine.UserAction{
			engine.UserActionApply, engine.UserActionApply, engine.UserActionApply,
		}}, true)

	var walker engine.Walker[int, string] = &engine.SequentialWalker[int, string]{Cell: handler, MaxFixes: 2}
	if concurrent {
		walker = &engine.ConcurrentWalker[int, string]{MaxFixes: 2,
			CellFor: func(int, string) *engine.CellHandler[int, string] { return handler }}
	}

	run, err := walker.Walk(t.Context(), 0, []string{"question"})

	want := []string{"Q", "F", "apply", "Q", "F", "apply", "Q"}
	if converges {
		want = append(want, "Q")

		if err != nil || !run.Passed {
			t.Fatalf("last allowed apply: run=%+v error=%v", run, err)
		}
	} else if !errors.Is(err, pkgerrors.ErrFixLoopStuck) {
		t.Fatalf("error=%v, want ErrFixLoopStuck", err)
	}

	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%v; want %v", events, want)
	}
}
