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
	// Waits marks a query that starts only once every item has answered every query before it.
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
	run        ItemRun[I]
	failed     []pendingCell[I, Q]
	err        error
	gateFailed bool
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

func (w *ConcurrentWalker[I, Q]) validateItems(
	ctx context.Context, items []I, queries []Q, onStart func(int, int),
) []validationRun[I, Q] {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if w.OnValidate != nil {
		w.OnValidate(items)
	}

	results := make([]validationRun[I, Q], len(items))
	for index, item := range items {
		results[index] = validationRun[I, Q]{run: ItemRun[I]{Item: item, Passed: true}}
	}

	for number, stage := range w.stages(queries) {
		if validationError(results) != nil {
			break
		}

		if number > 0 {
			onStart = nil
		}

		w.validateStage(ctx, cancel, items, queries[:stage[1]], stage[0], results, onStart)
	}

	return results
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
	for index := begin; index < len(queries); index++ {
		query := queries[index]

		if skippedByGate(result.gateFailed, w.Gate, w.OnSkip, item, query) {
			continue
		}

		gate := isGate(w.Gate, query)

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

		if !passed {
			result.run.Passed = false
			result.gateFailed = result.gateFailed || gate

			result.failed = append(result.failed, pendingCell[I, Q]{index: index, query: query, handler: handler})
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

		result, err := cell.handler.runFixLoop(ctx, run.Item, cell.query)
		if err != nil {
			return run, err
		}

		if result.Outcome == CellFixed {
			key := [2]int{index, cell.index}
			counts[key]++

			if counts[key] > fixLimit(w.MaxFixes) {
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
