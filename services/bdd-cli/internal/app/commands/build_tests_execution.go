package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	checklistloader "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

const defaultStepFixAttempts = 5

// buildTestsExecution verifies changed steps through the host's replay command.
// A package's helpers may affect every definition, so a changed steps package
// selects every bound Registry Scenario in that suite, including initially bound peers.
type buildTestsExecution struct {
	dispatcher *testrunner.Dispatcher
	loader     *checklistloader.ChecklistLoader
	maxFixes   int
	fixes      map[string]int
	arch       *architecture.Architecture
	scenarios  []*registry.RegistryScenario
	plan       *scenariogen.Plan
	root       string
	snapshots  map[string]string
	dirty      map[string]bool
	failures   []string
}

func (e *buildTestsExecution) prepare(
	arch *architecture.Architecture,
	scenarios []*registry.RegistryScenario,
	plan *scenariogen.Plan,
	root string,
) error {
	e.arch, e.scenarios, e.plan, e.root = arch, scenarios, plan, root
	e.dirty = make(map[string]bool)
	e.fixes = make(map[string]int)
	e.maxFixes = defaultStepFixAttempts

	doc, err := e.loader.LoadFull("build-tests")
	if err != nil {
		return fmt.Errorf("load fix attempt bound: %w", err)
	}

	if doc.Config != nil && doc.Config.MaxApplyAttempts > 0 {
		e.maxFixes = doc.Config.MaxApplyAttempts
	}

	err = validateSuites(e.dispatcher, arch.Testing)
	if err != nil {
		return err
	}

	snapshots, err := e.snapshot()
	if err != nil {
		return err
	}

	e.snapshots = snapshots

	return nil
}

func (e *buildTestsExecution) postFix(ctx context.Context, item *BuildTestsItem, _ string) (*BuildTestsItem, error) {
	drifted, err := scenariogen.Verify(e.plan, e.root)
	if err != nil {
		return item, err
	}

	if len(drifted) > 0 {
		return item, fmt.Errorf("%w: %s", ErrFixTouchedGeneratedTests, scenariogen.DriftReport(drifted))
	}

	err = e.changedSuites()
	if err != nil {
		return item, err
	}

	e.failures = nil

	answer, err := stepcoverage.Scan(e.scenarios, e.root)
	if err != nil {
		e.failures = []string{fmt.Sprintf("Scenario %s: step definitions cannot run: %s", item.ID, err)}
	} else {
		e.failures = e.runBound(ctx, answer)
	}

	item.Failures = e.failures
	for _, failure := range e.failures {
		slog.Error("Step execution failed", "scenario", item.ID, "failure", failure)
	}

	e.fixes[item.ID]++
	if len(e.failures) > 0 && e.fixes[item.ID] >= e.maxFixes {
		return item, fmt.Errorf("%w after %d fixes for Scenario %s:\n%s",
			ErrStepsStillFailing, e.maxFixes, item.ID, strings.Join(e.failures, "\n"))
	}

	return item, nil
}

func (e *buildTestsExecution) runBound(ctx context.Context, answer *stepcoverage.Answer) []string {
	var failures []string

	for _, file := range e.plan.Files {
		if !e.dirty[stepcoverage.StepsDir(file.Path)] {
			continue
		}

		names := make([]string, 0, len(file.Scenarios))

		ids := make([]string, 0, len(file.Scenarios))
		for _, scenario := range file.Scenarios {
			if answer.Examined[scenario.ID] && len(answer.Gaps[scenario.ID]) == 0 {
				names = append(names, scenario.FuncName)
				ids = append(ids, scenario.ID)
			}
		}

		if len(names) == 0 {
			continue
		}

		failures = append(failures, e.runSelection(ctx, file.Path, names, ids)...)
	}

	return failures
}

func (e *buildTestsExecution) runSelection(ctx context.Context, path string, names, ids []string) []string {
	cfg := testrunner.Config{Framework: e.arch.Testing.Framework, ConfigFile: e.arch.Testing.ConfigFile,
		Command: replayCommand(e.arch.Testing), Path: filepath.Dir(path), Tests: names}

	suiteRunner, err := e.dispatcher.For(cfg.Framework)
	if err != nil {
		return []string{err.Error()}
	}

	slog.Info("Running Scenarios using changed step definitions",
		"scenarios", strings.Join(ids, ","), "command", cfg.Command)

	failures, err := suiteRunner.Discover(ctx, cfg, "", cfg.Path)
	if err != nil {
		return []string{fmt.Sprintf("Scenarios %s: %s", strings.Join(ids, ", "), err)}
	}

	context := make([]string, 0, len(failures))
	for _, failure := range failures {
		context = append(context, fmt.Sprintf("Scenarios %s; test %s:\n%s",
			strings.Join(ids, ", "), failure.TestName, failure.FailureOutput))
	}

	return context
}

func (e *buildTestsExecution) reviewResult(
	_ context.Context,
	item *BuildTestsItem,
	result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	item.Failures = e.failures
	if len(e.failures) == 0 {
		return result, nil
	}

	result.Status = checklist.StatusFail
	result.ActualAnswer = "fail"
	result.Context = append(result.Context, e.failures...)
	result.FixPrompt = "Repair the failing step definitions from the runtime evidence. " +
		"Do not change the Registry, generated tests, suite command or scenario shim."

	return result, nil
}

func (e *buildTestsExecution) changedSuites() error {
	snapshots, err := e.snapshot()
	if err != nil {
		return err
	}

	for suite, hash := range snapshots {
		if hash != e.snapshots[suite] {
			e.dirty[suite] = true
		}
	}

	e.snapshots = snapshots

	return nil
}

// ErrStepsStillFailing names execution evidence when a fix walk stops.
var ErrStepsStillFailing = errors.New("authored step definitions still fail")

func (e *buildTestsExecution) finish(err error) error {
	if len(e.failures) == 0 || errors.Is(err, ErrStepsStillFailing) {
		return err
	}

	sort.Strings(e.failures)

	return errors.Join(err, fmt.Errorf("%w:\n%s", ErrStepsStillFailing, strings.Join(e.failures, "\n")))
}
