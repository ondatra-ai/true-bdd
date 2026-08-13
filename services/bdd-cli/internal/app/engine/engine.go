package engine

import (
	"context"
	"fmt"
)

// Engine renders prompts once via generateQ, iterates items, and
// drives the outer fixpoint re-walk. The Walker is injected so
// engine tests can stub it and future iteration policies (parallel,
// no-restart) drop in without touching the engine.
type Engine[I, P, Q any] struct {
	generateQ GenerateQFn[P, Q]
	walker    Walker[I, Q]
	opts      Options
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

// Run drives the (items × prompts) walk to a stop reason.
//
// Termination semantics:
//   - empty prompts → AllPassed=true, Reason=Converged.
//   - a walk that applies zero fixes and all items pass → Converged.
//   - a walk that applies zero fixes and some item fails → NotFixed.
//   - a walk that applies any fix triggers a re-walk (cross-item
//     interactions can invalidate previously-passing cells).
//   - any walker reporting UserExited short-circuits → UserExit.
//   - exhausting MaxApplyAttempts re-walks → MaxAttemptsExhausted.
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

	maxAttempts := e.opts.MaxApplyAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxApplyAttempts
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if e.opts.OnAttemptStart != nil {
			e.opts.OnAttemptStart(attempt, maxAttempts)
		}

		summary, err := e.walkAllItems(ctx, items, queries)
		if err != nil {
			return nil, err
		}

		if done := terminalResult(summary, items); done != nil {
			return done, nil
		}
	}

	return &Result[I]{Items: items, Reason: MaxAttemptsExhausted}, nil
}

// walkSummary captures one outer-walk pass: did every item pass, was
// any fix applied, did the user bail mid-walk.
type walkSummary struct {
	allPassed     bool
	anyFixApplied bool
	exit          bool
}

// terminalResult returns the Result to bail with after one walk, or
// nil if the engine should re-walk. The fixpoint contract: any fix
// triggers another walk (cross-item interactions); zero fixes means
// we've converged one way or the other.
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
