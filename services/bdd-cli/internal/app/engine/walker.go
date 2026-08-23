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
}

// Walk runs one item across all queries.
func (w *SequentialWalker[I, Q]) Walk(
	ctx context.Context,
	item I,
	queries []Q,
) (ItemRun[I], error) {
	out := ItemRun[I]{Item: item, Passed: true}

	maxFixes := w.MaxFixes
	if maxFixes <= 0 {
		maxFixes = defaultMaxApplyAttempts
	}

	fixesByQuery := make(map[int]int, len(queries))

	idx := 0
	for idx < len(queries) {
		result, err := w.Cell.Handle(ctx, out.Item, queries[idx])
		if err != nil {
			return out, err
		}

		switch result.Outcome {
		case CellPassed:
			idx++
		case CellFailedNoFix:
			out.Passed = false
			idx++
		case CellFixed:
			fixesByQuery[idx]++
			out.Item = result.Item
			out.FixApplied = true

			// Strictly greater, not >=: MaxFixes fixes are allowed and
			// each gets re-validated by the restart below, so failing on
			// the last one would reject a fix that actually worked.
			if fixesByQuery[idx] > maxFixes {
				return out, pkgerrors.ErrFixLoopNotConverging(fixesByQuery[idx])
			}

			idx = 0
		case CellUserExited:
			out.UserExited = true

			return out, nil
		}
	}

	return out, nil
}
