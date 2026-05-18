package engine

import "context"

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
}

// Walk runs one item across all queries.
func (w *SequentialWalker[I, Q]) Walk(
	ctx context.Context,
	item I,
	queries []Q,
) (ItemRun[I], error) {
	out := ItemRun[I]{Item: item, Passed: true}

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
			out.Item = result.Item
			out.FixApplied = true
			idx = 0
		case CellUserExited:
			out.UserExited = true

			return out, nil
		}
	}

	return out, nil
}
