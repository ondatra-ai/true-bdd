package engine

import (
	"context"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// Walker iterates queries for ONE item. The interface lets the engine
// stay agnostic to iteration policy — SequentialWalker restarts
// prompts from 0 on fix; other policies could plug in here.
type Walker[I, Q any] interface {
	Walk(ctx context.Context, item I, queries []Q) (ItemRun[I], error)
}

// SequentialWalker walks queries in index order. On CellFixed it
// restarts from index 0 so the fix is re-verified against earlier
// prompts that may have implicitly relied on the post-fix state.
type SequentialWalker[I, Q any] struct {
	Cell *CellHandler[I, Q]
	// MaxFixes bounds fixes for ONE query, not the whole walk (0 →
	// defaultMaxApplyAttempts) — per query because repeat-failing the
	// SAME check is the stuck symptom; see the check below for the +1.
	MaxFixes int
	// Gate, if non-nil, marks the queries whose failure skips the item's
	// other queries until a fix restarts it: a `D:` Prompt's Cells.
	Gate func(q Q) bool
	// OnSkip, if non-nil, is told of every query a failed gate skipped.
	OnSkip func(item I, q Q)
}

// itemWalk is one Walk's progress over one item's queries.
type itemWalk[I any] struct {
	out          ItemRun[I]
	idx          int
	gateFailed   bool
	fixesByQuery map[int]int
}

// Walk runs one item across all queries.
func (w *SequentialWalker[I, Q]) Walk(
	ctx context.Context,
	item I,
	queries []Q,
) (ItemRun[I], error) {
	walk := &itemWalk[I]{
		out:          ItemRun[I]{Item: item, Passed: true},
		fixesByQuery: make(map[int]int, len(queries)),
	}

	for walk.idx < len(queries) {
		query := queries[walk.idx]

		if walk.gateFailed && !w.isGate(query) {
			if w.OnSkip != nil {
				w.OnSkip(walk.out.Item, query)
			}

			walk.idx++

			continue
		}

		result, err := w.Cell.Handle(ctx, walk.out.Item, query)
		if err != nil {
			return walk.out, err
		}

		done, err := w.advance(walk, result, w.isGate(query))
		if err != nil || done {
			return walk.out, err
		}
	}

	return walk.out, nil
}

// advance applies one Cell's outcome to the walk, reporting whether the
// item's walk is over.
func (w *SequentialWalker[I, Q]) advance(walk *itemWalk[I], result CellResult[I], gate bool) (bool, error) {
	switch result.Outcome {
	case CellPassed:
		walk.idx++
	case CellFailedNoFix:
		walk.out.Passed = false
		walk.gateFailed = walk.gateFailed || gate
		walk.idx++
	case CellFixed:
		walk.fixesByQuery[walk.idx]++
		walk.out.Item = result.Item
		walk.out.FixApplied = true

		// Strictly greater, not >=: MaxFixes fixes are allowed and
		// each gets re-validated by the restart below, so failing on
		// the last one would reject a fix that actually worked.
		if walk.fixesByQuery[walk.idx] > w.maxFixes() {
			return true, pkgerrors.ErrFixLoopNotConverging(walk.fixesByQuery[walk.idx])
		}

		walk.idx = 0
		walk.gateFailed = false
	case CellUserExited:
		walk.out.UserExited = true

		return true, nil
	}

	return false, nil
}

// isGate reports whether a failure of the query skips the item's others.
func (w *SequentialWalker[I, Q]) isGate(query Q) bool {
	return w.Gate != nil && w.Gate(query)
}

// maxFixes is MaxFixes, defaulted.
func (w *SequentialWalker[I, Q]) maxFixes() int {
	if w.MaxFixes <= 0 {
		return defaultMaxApplyAttempts
	}

	return w.MaxFixes
}
