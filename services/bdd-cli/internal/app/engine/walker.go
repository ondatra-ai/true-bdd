package engine

import (
	"context"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// Walker iterates queries for ONE item. The interface lets the engine
// stay agnostic to iteration policy: SequentialWalker is the default
// (restart prompts from 0 on fix); future variants could run prompts
// in parallel or stop on first fix.
type Walker[I, Q any] interface {
	Walk(ctx context.Context, item I, queries []Q) (ItemRun[I], error)
}

// SequentialWalker walks queries in index order. On CellFixed it
// restarts from index 0 so the fix is re-verified against earlier
// prompts that may have implicitly relied on the post-fix state.
type SequentialWalker[I, Q any] struct {
	Cell *CellHandler[I, Q]
	// MaxFixes is the fix budget for ONE query, not for the walk. The
	// restart-on-fix rule above is otherwise unbounded: a fix that
	// reports success without changing anything leaves its check
	// failing, which applies another fix, forever. 0 →
	// defaultMaxApplyAttempts.
	//
	// Counted per query because repeatedly fixing the SAME check is the
	// symptom of a stuck loop, while fixing many different checks is
	// just a long walk making progress — a nine-prompt refine that
	// fixes each prompt once is healthy and must not trip the guard.
	//
	// The stop trips on the fix AFTER the budget, so one query gets at
	// most MaxFixes+1 fixes. That extra one is not slack: a fix is only
	// known to have failed once the restart re-validates it, and a
	// check asking for yet another fix after MaxFixes of them is the
	// evidence that it is not converging.
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

			// Strictly greater: MaxFixes fixes are allowed on this
			// query, and each gets re-validated by the restart below —
			// erroring on the last one would fail a fix that actually
			// worked. Only an attempt PAST the budget means this check
			// is not converging: something upstream is wrong (the fix
			// prompt, the applier's reach, or a verdict the fix cannot
			// move) and another identical round will not discover which.
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
