package engine

import (
	"context"
	"fmt"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

// Walker traverses one Item's Checklist items. A fix rechecks its own Q
// before restarting the checklist in written order.
type Walker[I, Q any] interface {
	Walk(ctx context.Context, item I, queries []Q) (ItemRun[I], error)
}

// SequentialWalker walks written order, rechecking a fixed query before
// restarting the checklist to detect regressions in earlier queries.
type SequentialWalker[I, Q any] struct {
	Cell *CellHandler[I, Q]
	// MaxFixes bounds applies per Checklist item (0 → defaultMaxApplyAttempts).
	// Every apply is rechecked, including the last allowed one.
	MaxFixes int
	// CellLabel names the bounded Cell in an exhaustion error.
	CellLabel func(I, Q) string
}

// itemWalk is one Walk's progress over one item's queries.
type itemWalk[I any] struct {
	out              ItemRun[I]
	idx              int
	restartAfterPass bool
	fixesByQuery     map[int]int
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

		handler := *w.Cell
		handler.FixMode = w.Cell.FixMode && walk.fixesByQuery[walk.idx] < fixLimit(w.MaxFixes)

		result, err := handler.Handle(ctx, walk.out.Item, query)
		if err != nil {
			return walk.out, err
		}

		done, err := w.advance(walk, result)
		if err != nil || done {
			if err != nil && w.CellLabel != nil {
				err = fmt.Errorf("%s: %w", w.CellLabel(walk.out.Item, query), err)
			}

			return walk.out, err
		}
	}

	return walk.out, nil
}

// advance applies one Cell's outcome to the walk, reporting whether the
// item's walk is over.
func (w *SequentialWalker[I, Q]) advance(walk *itemWalk[I], result CellResult[I]) (bool, error) {
	switch result.Outcome {
	case CellPassed:
		if walk.restartAfterPass {
			walk.idx = 0
			walk.restartAfterPass = false
			walk.out.Passed = true
		} else {
			walk.idx++
		}
	case CellFailedNoFix:
		if w.Cell.FixMode && walk.fixesByQuery[walk.idx] >= fixLimit(w.MaxFixes) {
			return true, pkgerrors.ErrFixLoopNotConverging(walk.fixesByQuery[walk.idx])
		}

		walk.restartAfterPass = false
		walk.out.Passed = false
		walk.idx++
	case CellFixed:
		walk.fixesByQuery[walk.idx]++
		walk.out.Item = result.Item
		walk.out.FixApplied = true

		walk.restartAfterPass = true
	case CellUserExited:
		walk.out.UserExited = true

		return true, nil
	}

	return false, nil
}
