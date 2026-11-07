package engine_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
)

func TestConcurrentValidationRepairsBeforeAdvancingAndDiscardsStaleFixes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var guard sync.Mutex

	starts := make(chan int, 2)
	release := make(chan struct{})

	go func() { <-starts; <-starts; close(release) }()

	fixed := false

	var menus []int

	walker := &engine.ConcurrentWalker[int, string]{Limit: 2}
	walker.CellFor = func(item int, query string) *engine.CellHandler[int, string] {
		return &engine.CellHandler[int, string]{FixMode: true,
			UI: &stubUI{actionsScript: []engine.UserAction{engine.UserActionApply}},
			Query: func(ctx context.Context, _ int, _ string) (bool, error) {
				guard.Lock()

				nowFixed := fixed
				guard.Unlock()

				if query == "d" {
					return item != 3, nil
				}


				if !nowFixed {
					starts <- item

					select {
					case <-release:
					case <-ctx.Done():
						return false, ctx.Err()
					}
				}

				return nowFixed, nil
			},
			GenFix: func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
				guard.Lock()
				defer guard.Unlock()

				menus = append(menus, item)
				if item == 3 {
					return engine.FixResult{}, nil
				}

				return engine.FixResult{FixPrompt: orderFix}, nil
			},
			Fix: func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
				guard.Lock()
				fixed = true
				guard.Unlock()

				return item, nil
			},
		}
	}
	eng := engine.New(func(_ int, q string) string { return q }, walker, engine.Options{})

	result, err := eng.Run(ctx, []int{1, 2, 3}, []string{"d", "q"})
	if err != nil {
		t.Fatal(err)
	}

	if result.Reason != engine.NotFixed {
		t.Fatalf("reason %v", result.Reason)
	}

	if fmt.Sprint(menus) != "[1 3 3]" {
		t.Fatalf("stale/menu order: %v", menus)
	}
}

func TestConcurrentWalkAllowsRepairsToMoreThanFiveScenarios(t *testing.T) {
	t.Parallel()

	var guard sync.Mutex

	repaired := map[int]bool{}
	user := &stubUI{actionsScript: []engine.UserAction{
		engine.UserActionApply, engine.UserActionApply, engine.UserActionApply,
		engine.UserActionApply, engine.UserActionApply, engine.UserActionApply,
	}}
	walker := &engine.ConcurrentWalker[int, string]{Limit: 2,
		CellFor: func(_ int, _ string) *engine.CellHandler[int, string] {
			return &engine.CellHandler[int, string]{FixMode: true, UI: user,
				Query: func(_ context.Context, item int, _ string) (bool, error) {
					guard.Lock()
					defer guard.Unlock()

					return repaired[item], nil
				},
				GenFix: func(_ context.Context, _ int, _ string, _ map[string]string, _ int) (engine.FixResult, error) {
					return engine.FixResult{FixPrompt: orderFix}, nil
				},
				Fix: func(_ context.Context, item int, _ engine.FixDecision) (int, error) {
					repaired[item] = true

					return item, nil
				},
			}
		}}
	eng := engine.New(func(_ int, q string) string { return q }, walker, engine.Options{})

	result, err := eng.Run(context.Background(), []int{1, 2, 3, 4, 5, 6}, []string{"q"})
	if err != nil {
		t.Fatal(err)
	}

	if result.Reason != engine.Converged || len(repaired) != 6 {
		t.Fatalf("reason=%v, repaired=%v", result.Reason, repaired)
	}
}

func TestConcurrentWalkerHonorsValidationLimit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	entered := make(chan struct{}, 5)
	release := make(chan struct{})

	var active, peak atomic.Int32

	walker := &engine.ConcurrentWalker[int, string]{Limit: 2,
		CellFor: func(_ int, _ string) *engine.CellHandler[int, string] {
			return &engine.CellHandler[int, string]{Query: func(ctx context.Context, _ int, _ string) (bool, error) {
				count := active.Add(1)
				defer active.Add(-1)

				for previous := peak.Load(); count > previous; previous = peak.Load() {
					if peak.CompareAndSwap(previous, count) {
						break
					}
				}

				entered <- struct{}{}

				select {
				case <-release:
					return true, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}}
		},
	}
	done := make(chan error, 1)

	go func() { _, err := walker.WalkItems(ctx, []int{1, 2, 3, 4, 5}, []string{"q"}, nil); done <- err }()

	for range 2 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("two Scenarios did not overlap")
		}
	}

	close(release)

	err := <-done
	if err != nil {
		t.Fatal(err)
	}

	if peak.Load() != 2 {
		t.Fatalf("peak=%d, want 2", peak.Load())
	}
}

func TestConcurrentWalkerStartsAWaitingPromptOnlyAfterEveryItemsEarlierPrompts(t *testing.T) {
	t.Parallel()

	var guard sync.Mutex

	var order, walks []string

	walker := &engine.ConcurrentWalker[int, string]{Limit: 2,
		Waits: func(query string) bool { return query == "bindings" },
		OnValidate: func(items []int) { walks = append(walks, fmt.Sprint(items)) },
		CellFor: func(_ int, _ string) *engine.CellHandler[int, string] {
			return &engine.CellHandler[int, string]{Query: func(_ context.Context, item int, query string) (bool, error) {
				guard.Lock()
				defer guard.Unlock()

				order = append(order, fmt.Sprintf("%s-%d", query, item))

				return true, nil
			}}
		},
	}

	runs, err := walker.WalkItems(t.Context(), []int{1, 2, 3, 4, 5}, []string{"d", "link", "bindings", "words"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(runs) != 5 || len(order) != 20 || fmt.Sprint(walks) != "[[1 2 3 4 5]]" {
		t.Fatalf("runs=%d cells=%v walks=%v", len(runs), order, walks)
	}

	assertLinksFirst(t, order, 10)
}

// assertLinksFirst holds the first cells to d and link, and every later one to bindings and words.
func assertLinksFirst(t *testing.T, order []string, links int) {
	t.Helper()

	for index, cell := range order {
		early := cell[0] == 'd' || cell[0] == 'l'
		if early != (index < links) {
			t.Fatalf("cell %s at %d: a waiting Prompt began before every link answered: %v", cell, index, order)
		}
	}
}
