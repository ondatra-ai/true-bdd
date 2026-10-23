package commands

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

// buildCodeExecution invalidates every stored verdict after any apply turn.
type buildCodeExecution struct {
	deps       BuildCodeDeps
	load       func(context.Context) ([]*testrunner.FailingTest, error)
	generation int
	verdicts   map[*testrunner.FailingTest]int
}

func newBuildCodeExecution(deps BuildCodeDeps, architectureFile string) *buildCodeExecution {
	return &buildCodeExecution{
		deps:     deps,
		load:     loadFailingTests(deps, architectureFile),
		verdicts: make(map[*testrunner.FailingTest]int),
	}
}

func (e *buildCodeExecution) discover(ctx context.Context) ([]*testrunner.FailingTest, error) {
	items, err := e.load(ctx)
	if err != nil {
		return nil, err
	}

	clear(e.verdicts)

	for _, item := range items {
		e.verdicts[item] = e.generation
	}

	return items, nil
}

func (e *buildCodeExecution) beforeEvaluate(ctx context.Context, item *testrunner.FailingTest) (bool, error) {
	if generation, known := e.verdicts[item]; !known || generation != e.generation {
		runnerImpl, err := e.deps.TestRunnerDispatcher.For(item.Framework)
		if err != nil {
			return false, fmt.Errorf("rerun dispatch %s: %w", item.Framework, err)
		}

		passed, output, err := runnerImpl.RunOne(ctx, item)
		if err != nil {
			return false, fmt.Errorf("rerun %s: %w", item.ID, err)
		}

		item.LastRunPassed = passed
		item.FailureOutput = testrunner.TruncateTail(output, testrunner.FailureOutputCap)
		item.LastRunAt = time.Now()
		e.verdicts[item] = e.generation
	}

	return item.LastRunPassed, nil
}

func (e *buildCodeExecution) postFix(
 _ context.Context, item *testrunner.FailingTest, _ string,
) (*testrunner.FailingTest, error) {
	slog.Info("Fix applied; re-running this test in isolation")

	e.generation++

	return item, nil
}

func (e *buildCodeExecution) postNotApplied(
 _ context.Context, item *testrunner.FailingTest,
) (*testrunner.FailingTest, error) {
	e.generation++

	return item, nil
}
