package engine

import (
	"context"
	"fmt"
	"sync"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// ConcurrentWalker validates items in parallel, then repairs in item order.
// Each apply discards outstanding verdicts and rejudges every item.
type ConcurrentWalker[I, Q any] struct {
	Limit    int
	MaxFixes int
	CellFor  func(I, Q) *CellHandler[I, Q]
	Gate     func(Q) bool
	OnSkip   func(I, Q)
}

type pendingCell[I, Q any] struct {
	index   int
	query   Q
	handler *CellHandler[I, Q]
}

type validationRun[I, Q any] struct {
	run    ItemRun[I]
	failed []pendingCell[I, Q]
	err    error
}

func (w *ConcurrentWalker[I, Q]) Walk(ctx context.Context, item I, queries []Q) (ItemRun[I], error) {
	runs, err := w.WalkItems(ctx, []I{item}, queries, nil)
	if err != nil {
		return ItemRun[I]{Item: item}, err
	}

	return runs[0], nil
}

func (w *ConcurrentWalker[I, Q]) WalkItems(
	ctx context.Context, items []I, queries []Q, onStart func(int, int),
) ([]ItemRun[I], error) {
	counts := make(map[[2]int]int)

	applied := make([]bool, len(items))
	for {
		validated := w.validateItems(ctx, items, queries, onStart)

		err := validationError(validated)
		if err != nil {
			return nil, err
		}

		runs, changed, err := w.repairItems(ctx, validated, counts)
		if err != nil {
			return nil, err
		}

		for index, run := range runs {
			items[index] = run.Item
			applied[index] = applied[index] || run.FixApplied
		}

		if changed {
			continue
		}

		for index := range runs {
			runs[index].FixApplied = applied[index]
		}

		return runs, nil
	}
}

func validationError[I, Q any](validated []validationRun[I, Q]) error {
	for index, result := range validated {
		if result.err != nil {
			return fmt.Errorf("validation failed on item %d: %w", index, result.err)
		}
	}

	return nil
}

func (w *ConcurrentWalker[I, Q]) repairItems(
	ctx context.Context, validated []validationRun[I, Q], counts map[[2]int]int,
) ([]ItemRun[I], bool, error) {
	runs := make([]ItemRun[I], 0, len(validated))
	for index, validation := range validated {
		run, err := w.repair(ctx, validation, index, counts)
		if err != nil {
			return nil, false, err
		}

		runs = append(runs, run)
		if run.FixApplied {
			return runs, true, nil
		}

		if run.UserExited {
			return runs, false, nil
		}
	}

	return runs, false, nil
}

func (w *ConcurrentWalker[I, Q]) validateItems(
	ctx context.Context, items []I, queries []Q, onStart func(int, int),
) []validationRun[I, Q] {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]validationRun[I, Q], len(items))
	jobs := make(chan int)

	var group sync.WaitGroup
	for range min(max(1, w.Limit), len(items)) {
		group.Go(func() {
			for index := range jobs {
				results[index] = w.validate(ctx, items[index], queries)
				if results[index].err != nil {
					cancel()
				}
			}
		})
	}

	for index := range items {
		if onStart != nil {
			onStart(index, len(items))
		}

		jobs <- index
	}

	close(jobs)
	group.Wait()

	return results
}

func (w *ConcurrentWalker[I, Q]) validate(ctx context.Context, item I, queries []Q) validationRun[I, Q] {
	result := validationRun[I, Q]{run: ItemRun[I]{Item: item, Passed: true}}
	gateFailed := false

	for index, query := range queries {
		gate := w.Gate != nil && w.Gate(query)
		if gateFailed && !gate {
			if w.OnSkip != nil {
				w.OnSkip(item, query)
			}

			continue
		}

		contextErr := ctx.Err()
		if contextErr != nil {
			result.err = contextErr

			return result
		}

		handler := w.CellFor(item, query)

		passed, err := handler.Query(ctx, item, query)
		if err != nil {
			result.err = err

			return result
		}

		if !passed {
			result.run.Passed = false
			gateFailed = gateFailed || gate

			result.failed = append(result.failed, pendingCell[I, Q]{index: index, query: query, handler: handler})
		}
	}

	return result
}

func (w *ConcurrentWalker[I, Q]) repair(
	ctx context.Context, validation validationRun[I, Q], index int, counts map[[2]int]int,
) (ItemRun[I], error) {
	run := validation.run
	for _, cell := range validation.failed {
		if !cell.handler.FixMode {
			continue
		}

		result, err := cell.handler.runFixLoop(ctx, run.Item, cell.query)
		if err != nil {
			return run, err
		}

		if result.Outcome == CellFixed {
			key := [2]int{index, cell.index}
			counts[key]++

			limit := w.MaxFixes
			if limit <= 0 {
				limit = defaultMaxApplyAttempts
			}

			if counts[key] > limit {
				return run, pkgerrors.ErrFixLoopNotConverging(counts[key])
			}

			run.Item = result.Item
			run.FixApplied = true

			return run, nil
		}

		if result.Outcome == CellUserExited {
			run.UserExited = true

			return run, nil
		}
	}

	return run, nil
}
