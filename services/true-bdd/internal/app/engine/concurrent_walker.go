package engine

import (
	"context"
	"fmt"
	"sync"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// ConcurrentWalker walks Items concurrently in written order, pausing each at
// its first fixable failure and repairing in Item order before restarting.
type ConcurrentWalker[I, Q any] struct {
	Limit    int
	MaxFixes int
	CellFor  func(I, Q) *CellHandler[I, Q]
	// BeforeRecheck invalidates shared verdicts after each apply, keeping the current links.
	BeforeRecheck func()
	// Waits marks a query whose predecessors every Item must finish before it starts.
	Waits func(Q) bool
	// OnValidate, if non-nil, is told each validation round's items before its first cell.
	OnValidate func([]I)
}

type pendingCell[I, Q any] struct {
	index   int
	query   Q
	handler *CellHandler[I, Q]
}

type validationRun[I, Q any] struct {
	next   int
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
		runs, changed, err := w.walkRound(ctx, items, queries, onStart, counts)
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
			return fmt.Errorf("walker failed on item %d: %w", index, result.err)
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

// walkRound retains barriers for shared judgements while allowing independent Item progress.
func (w *ConcurrentWalker[I, Q]) walkRound(
	ctx context.Context, items []I, queries []Q, onStart func(int, int), counts map[[2]int]int,
) ([]ItemRun[I], bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if w.OnValidate != nil {
		w.OnValidate(items)
	}

	results := make([]validationRun[I, Q], len(items))

	runs := make([]ItemRun[I], len(items))
	for index, item := range items {
		runs[index] = ItemRun[I]{Item: item, Passed: true}
		results[index].run = runs[index]
	}

	for _, stage := range w.stages(queries) {
		var (
			changed bool
			err     error
		)

		runs, changed, err = w.walkStage(ctx, cancel, items, queries[:stage[1]], stage[0], results, onStart, counts)
		if err != nil || changed {
			return runs, changed, err
		}

		for _, run := range runs {
			if run.UserExited {
				return runs, false, nil
			}
		}

		onStart = nil
	}

	return runs, false, nil
}

func (w *ConcurrentWalker[I, Q]) walkStage(
	ctx context.Context, cancel context.CancelFunc, items []I, queries []Q, begin int,
	results []validationRun[I, Q], onStart func(int, int), counts map[[2]int]int,
) ([]ItemRun[I], bool, error) {
	for {
		w.validateStage(ctx, cancel, items, queries, begin, results, onStart)
		onStart = nil

		err := validationError(results)
		if err != nil {
			return nil, false, err
		}

		runs, changed, err := w.repairItems(ctx, results, counts)
		if err != nil || changed {
			return runs, changed, err
		}

		more := false

		for index, run := range runs {
			if run.UserExited {
				return runs, false, nil
			}

			results[index].run, results[index].failed = run, nil
			more = more || results[index].next < len(queries)
		}

		if !more {
			return runs, false, nil
		}
	}
}

// stages splits the queries before every waiting one, as [begin, end) ranges.
func (w *ConcurrentWalker[I, Q]) stages(queries []Q) [][2]int {
	stages := make([][2]int, 0, 1)

	begin := 0
	for index, query := range queries {
		if index > begin && w.Waits != nil && w.Waits(query) {
			stages = append(stages, [2]int{begin, index})
			begin = index
		}
	}

	return append(stages, [2]int{begin, len(queries)})
}

func (w *ConcurrentWalker[I, Q]) validateStage(
	ctx context.Context, cancel context.CancelFunc, items []I, queries []Q, begin int,
	results []validationRun[I, Q], onStart func(int, int),
) {
	jobs := make(chan int)

	var group sync.WaitGroup
	for range min(max(1, w.Limit), len(items)) {
		group.Go(func() {
			for index := range jobs {
				w.validate(ctx, items[index], queries, begin, &results[index])

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
}

func (w *ConcurrentWalker[I, Q]) validate(
	ctx context.Context, item I, queries []Q, begin int, result *validationRun[I, Q],
) {
	for index := max(begin, result.next); index < len(queries); index++ {
		query := queries[index]

		contextErr := ctx.Err()
		if contextErr != nil {
			result.err = contextErr

			return
		}

		handler := w.CellFor(item, query)

		passed, err := handler.Query(ctx, item, query)
		if err != nil {
			result.err = err

			return
		}

		result.next = index + 1

		if !passed {
			result.run.Passed = false

			result.failed = append(result.failed, pendingCell[I, Q]{index: index, query: query, handler: handler})
			if handler.FixMode {
				return
			}
		}
	}
}

func (w *ConcurrentWalker[I, Q]) repair(
	ctx context.Context, validation validationRun[I, Q], index int, counts map[[2]int]int,
) (ItemRun[I], error) {
	run := validation.run
	for _, cell := range validation.failed {
		if !cell.handler.FixMode {
			continue
		}

		var err error

		run, err = w.repairCell(ctx, run, cell, index, counts)
		if err != nil || run.FixApplied || run.UserExited {
			return run, err
		}
	}

	return run, nil
}

// repairCell retries the failed Q immediately, then returns to the checklist's start on success.
func (w *ConcurrentWalker[I, Q]) repairCell(
	ctx context.Context, run ItemRun[I], cell pendingCell[I, Q], index int, counts map[[2]int]int,
) (ItemRun[I], error) {
	key := [2]int{index, cell.index}

	for {
		if counts[key] >= fixLimit(w.MaxFixes) {
			return run, pkgerrors.ErrFixLoopNotConverging(counts[key])
		}

		result, err := cell.handler.runFixLoop(ctx, run.Item, cell.query)
		if err != nil {
			return run, err
		}

		switch result.Outcome {
		case CellFixed:
			counts[key]++

			run.Item, run.FixApplied = result.Item, true

			if w.BeforeRecheck != nil {
				w.BeforeRecheck()
			}

			passed, queryErr := cell.handler.Query(ctx, run.Item, cell.query)
			if queryErr != nil {
				return run, queryErr
			}

			if passed {
				return run, nil
			}
		case CellUserExited:
			run.UserExited = true

			return run, nil
		case CellPassed, CellFailedNoFix:
			return run, nil
		}
	}
}
