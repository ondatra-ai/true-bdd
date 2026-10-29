package engine

import (
	"context"
	"fmt"
)

// Engine renders prompts once via generateQ, iterates items, and
// drives the outer fixpoint re-walk. The Walker is injected so tests
// can stub it and other iteration policies can drop in later.
type Engine[I, P, Q any] struct {
	generateQ GenerateQFn[P, Q]
	walker    Walker[I, Q]
	opts      Options
	// ReloadItems, when set, discovers the next walk after a fix.
	ReloadItems func(context.Context) ([]I, error)
	// CheckConvergence may return findings to the same bounded walk.
	CheckConvergence func(context.Context, []I) (bool, error)
}

// New constructs an Engine.
func New[I, P, Q any](
	generateQ GenerateQFn[P, Q],
	walker Walker[I, Q],
	opts Options,
) *Engine[I, P, Q] {
	return &Engine[I, P, Q]{
		generateQ: generateQ,
		walker:    walker,
		opts:      opts,
	}
}

// Run drives the (items × prompts) walk to a stop reason. Termination
// semantics: zero prompts converges immediately, exhausting
// MaxApplyAttempts stops; see terminalResult for the fixpoint logic.
func (e *Engine[I, P, Q]) Run(
	ctx context.Context,
	items []I,
	prompts []P,
) (*Result[I], error) {
	if len(prompts) == 0 {
		return &Result[I]{Items: items, AllPassed: true, Reason: Converged}, nil
	}

	queries := make([]Q, len(prompts))
	for i, prompt := range prompts {
		queries[i] = e.generateQ(i+1, prompt)
	}

	maxAttempts := e.attemptLimit()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if e.opts.OnAttemptStart != nil {
			e.opts.OnAttemptStart(attempt, maxAttempts)
		}

		if attempt > 1 {
			var err error

			items, err = e.reload(ctx, items)
			if err != nil {
				return nil, fmt.Errorf("reload items: %w", err)
			}
		}

		summary, err := e.walkAllItems(ctx, items, queries)
		if err != nil {
			return nil, err
		}

		done, finished, err := e.completedWalk(ctx, summary, items)
		if err != nil {
			return nil, err
		}

		if finished {
			return &done, nil
		}
	}

	return &Result[I]{Items: items, Reason: MaxAttemptsExhausted}, nil
}

// reload preserves the current items for commands without fresh discovery.
func (e *Engine[I, P, Q]) reload(ctx context.Context, items []I) ([]I, error) {
	if e.ReloadItems == nil {
		return items, nil
	}

	return e.ReloadItems(ctx)
}

// walkSummary captures one outer-walk pass: did every item pass, was
// any fix applied, did the user bail mid-walk.
type walkSummary struct {
	allPassed     bool
	anyFixApplied bool
	exit          bool
}

// terminalResult returns the Result to bail with after one walk, or
// nil if the engine should re-walk: any fix triggers another walk
// (cross-item interactions); zero fixes means we've converged.
func terminalResult[I any](summary walkSummary, items []I) *Result[I] {
	switch {
	case summary.exit:
		return &Result[I]{Items: items, Reason: UserExit}
	case summary.allPassed && !summary.anyFixApplied:
		return &Result[I]{Items: items, AllPassed: true, Reason: Converged}
	case !summary.allPassed && !summary.anyFixApplied:
		return &Result[I]{Items: items, Reason: NotFixed}
	}

	return nil
}

// walkAllItems runs one outer walk pass over every item, mutating
// items[i] when a walker reports a fix-applied run. Returns a summary
// the engine uses to decide convergence / re-walk / abort.
func (e *Engine[I, P, Q]) walkAllItems(
	ctx context.Context,
	items []I,
	queries []Q,
) (walkSummary, error) {
	summary := walkSummary{allPassed: true}

	for idx, item := range items {
		if e.opts.OnItemStart != nil {
			e.opts.OnItemStart(idx, len(items))
		}

		run, err := e.walker.Walk(ctx, item, queries)
		if err != nil {
			return summary, fmt.Errorf("walker failed on item %d: %w", idx, err)
		}

		items[idx] = run.Item

		if !run.Passed {
			summary.allPassed = false
		}

		if run.FixApplied {
			summary.anyFixApplied = true
		}

		if run.UserExited {
			summary.exit = true

			break
		}
	}

	return summary, nil
}

func (e *Engine[I, P, Q]) completedWalk(ctx context.Context, summary walkSummary, items []I) (Result[I], bool, error) {
	if summary.allPassed && !summary.exit && e.CheckConvergence != nil {
		passed, err := e.CheckConvergence(ctx, items)

		return Result[I]{Items: items, AllPassed: passed, Reason: Converged}, passed, err
	}

	done := terminalResult(summary, items)
	if done == nil {
		return Result[I]{}, false, nil
	}

	return *done, true, nil
}

func (e *Engine[I, P, Q]) attemptLimit() int {
	if e.opts.MaxApplyAttempts > 0 {
		return e.opts.MaxApplyAttempts
	}

	return defaultMaxApplyAttempts
}
