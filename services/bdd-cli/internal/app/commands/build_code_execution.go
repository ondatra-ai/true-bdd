package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

// buildCodeExecution discovers one set and refreshes it after every completed apply.
type buildCodeExecution struct {
	deps             BuildCodeDeps
	load             func(context.Context) ([]*testrunner.FailingTest, error)
	architectureFile string
	generation       int
	verdicts         map[*BuildCodeItem]int
}

func newBuildCodeExecution(deps BuildCodeDeps, architectureFile string, fix bool) *buildCodeExecution {
	return &buildCodeExecution{
		deps: deps, load: loadFailingTests(deps, architectureFile, fix),
		verdicts:         make(map[*BuildCodeItem]int),
		architectureFile: architectureFile,
	}
}

func (e *buildCodeExecution) discover(ctx context.Context) ([]*BuildCodeItem, error) {
	tests, err := e.load(ctx)
	if err != nil {
		return nil, err
	}

	clear(e.verdicts)

	if len(tests) == 0 {
		return nil, nil
	}

	subjectID, title := "failing-set", "Failing set"
	if len(tests) == 1 {
		subjectID, title = testrunner.Subject(tests[0])
	}

	documents := map[string]string{"architecture_yaml": e.architectureFile}
	for _, key := range []string{"product", "scenarios_yaml"} {
		if path := optionalCodeDocument(e.deps.DocResolver, key); path != "" {
			documents[key] = path
		}
	}

	item := &BuildCodeItem{Tests: tests, Documents: documents, id: subjectID, title: title}

	e.verdicts[item] = e.generation

	return []*BuildCodeItem{item}, nil
}

func (e *buildCodeExecution) beforeEvaluate(ctx context.Context, item *BuildCodeItem) (bool, error) {
	_, err := e.refresh(ctx, item)
	if err != nil {
		return false, err
	}

	return false, e.configureWriter(item)
}

func (e *buildCodeExecution) refresh(ctx context.Context, item *BuildCodeItem) (*BuildCodeItem, error) {
	if generation, known := e.verdicts[item]; known && generation == e.generation {
		return item, nil
	}

	return e.rerun(ctx, item)
}

func (e *buildCodeExecution) rerun(ctx context.Context, item *BuildCodeItem) (*BuildCodeItem, error) {
	impl, err := e.deps.TestRunnerDispatcher.For(item.Tests[0].Framework)
	if err != nil {
		return item, fmt.Errorf("rerun dispatch: %w", err)
	}

	err = impl.RunSet(ctx, item.Tests)
	if err != nil {
		return item, fmt.Errorf("rerun failing set: %w", err)
	}

	remaining := make([]*testrunner.FailingTest, 0, len(item.Tests))
	for _, test := range item.Tests {
		if !test.LastRunPassed {
			remaining = append(remaining, test)
		}
	}

	item.Tests = remaining
	e.verdicts[item] = e.generation

	return item, nil
}

func (e *buildCodeExecution) postFix(ctx context.Context, item *BuildCodeItem, _ string) (*BuildCodeItem, error) {
	slog.Info("Fix applied; re-running the failing set")

	e.generation++

	return e.refresh(ctx, item)
}

func (e *buildCodeExecution) postNotApplied(ctx context.Context, item *BuildCodeItem) (*BuildCodeItem, error) {
	e.generation++

	return e.refresh(ctx, item)
}

func (e *buildCodeExecution) stopImpossible(ctx context.Context, item *BuildCodeItem, obstruction error) error {
	_, err := e.postNotApplied(ctx, item)
	if err != nil {
		return errors.Join(obstruction, err)
	}

	remaining := make([]string, 0, len(item.Tests))
	for _, test := range item.Tests {
		remaining = append(remaining, test.ID)
	}

	slog.Error("Repair impossible", "error", obstruction, "still_failing", remaining)

	remainingText := strings.Join(remaining, ", ")
	if len(remaining) == 0 {
		remainingText = "none"
	}

	return fmt.Errorf("%w; still failing after rerun: %s", obstruction, remainingText)
}

func (e *buildCodeExecution) configureWriter(item *BuildCodeItem) error {
	if len(item.Tests) == 0 {
		return nil
	}

	arch, err := architecture.Load(e.architectureFile)
	if err != nil {
		return fmt.Errorf("reload writer architecture: %w", err)
	}

	roots, err := codeWritableRoots(e.deps, arch)
	if err != nil {
		return err
	}

	configureCodeRoots(e.deps, codeRootsForSet(roots, arch.Services, item.Tests))

	return nil
}
