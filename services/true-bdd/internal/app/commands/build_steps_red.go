package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// ErrRedCheck distinguishes infrastructure failures from executed step failures.
var ErrRedCheck = errors.New("red without target check failed")

// ErrScenarioGreen distinguishes definitions doing the product's work.
var ErrScenarioGreen = errors.New("scenario green without target")

func (e *buildStepsExecution) checkConvergence(_ context.Context, _ []*BuildStepsItem) (bool, error) {
	scenarios, err := e.deps.RegistryLoader.Load(e.registryFile)
	if err != nil {
		return false, fmt.Errorf("reload Registry: %w", err)
	}

	identities, err := e.checkWholeSuite(scenarios)
	if err != nil {
		return false, err
	}

	verdict, err := e.runWithoutTarget(identities)
	if err != nil {
		return false, err
	}

	nameRedResults(&verdict, scenarios, identities)

	if len(verdict.Invalid) > 0 {
		slog.Error("Red without target check failed", "findings", verdict.Invalid)

		return false, fmt.Errorf("%w: %s", ErrRedCheck, strings.Join(verdict.Invalid, "\n"))
	}

	e.green = verdict.Green
	if len(verdict.Green) > 0 {
		return false, e.returnGreenItems(scenarios, identities, verdict.Green)
	}

	e.redChecked = true

	slog.Info("Red without target passed", "scenarios", len(identities))

	return true, nil
}

func (e *buildStepsExecution) checkWholeSuite(scenarios []*registry.RegistryScenario) ([]string, error) {
	answer, err := stepcoverage.Scan(scenarios, ".")
	if err != nil {
		return nil, err
	}

	if findings := stepCoverageFindings(answer); len(findings) > 0 {
		return nil, fmt.Errorf("%w: %v", ErrMissingStepDefinitions, findings)
	}

	slog.Info("Step coverage passed", "examined", len(answer.Examined))

	identities := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		identity, err := scenariogen.TestIdentity(".", scenario)
		if err != nil {
			return nil, err
		}

		identities = append(identities, identity)
	}

	err = e.compiler.ListScenarios(e.command, identities)
	if err != nil {
		return nil, err
	}

	slog.Info("Scenario list passed", "scenarios", len(identities))

	return identities, nil
}

func (e *buildStepsExecution) runWithoutTarget(identities []string) (testrunner.RedRun, error) {
	arch, err := architecture.Load(e.architectureFile)
	if err != nil {
		return testrunner.RedRun{}, err
	}

	host, err := os.Getwd()
	if err != nil {
		return testrunner.RedRun{}, fmt.Errorf("locate Host project: %w", err)
	}

	parent := filepath.Join(host, "tmp")

	err = disk.Dir(parent, disk.Shared)
	if err != nil {
		return testrunner.RedRun{}, fmt.Errorf("prepare red run: %w", err)
	}

	copyRoot, err := disk.TempDir(parent, "red-without-target-")
	if err != nil {
		return testrunner.RedRun{}, fmt.Errorf("prepare red run: %w", err)
	}
	defer func() {
		err := disk.RemoveTree(copyRoot)
		if err != nil {
			slog.Warn("Remove red-run copy", "error", err)
		}
	}()

	paths := make([]string, 0, len(arch.Services))
	for _, service := range arch.Services {
		paths = append(paths, service.Path)
	}

	err = testrunner.CopyWithoutTarget(host, copyRoot, paths)
	if err != nil {
		return testrunner.RedRun{}, err
	}

	cfg, err := testrunner.CopiedConfig(e.command, host, copyRoot)
	if err != nil {
		return testrunner.RedRun{}, err
	}

	return e.compiler.RedScenarios(cfg, identities)
}

func (e *buildStepsExecution) returnGreenItems(
	scenarios []*registry.RegistryScenario, identities, green []string,
) error {
	source, err := product.LoadCatalogue(productDocument(e.deps.DocResolver))
	if err != nil {
		return err
	}

	compiled, _ := catalogue.Compile(source.Entries, source.Roles)
	finding := fmt.Sprintf("%v: %s; a definition is doing the product's work. "+
		"Repair the written Items together through the declared Public interface.",
		ErrScenarioGreen, strings.Join(green, ", "))
	affected := make(map[*BuildStepsItem]bool)

	for index, scenario := range scenarios {
		if !slices.Contains(green, identities[index]) {
			continue
		}

		found := e.markGreenItems(scenario, compiled, affected)

		if !found {
			slog.Error("Scenario green without target", "scenario", scenario.ID,
				"finding", "no phrase used by this Scenario was written in this run")

			return fmt.Errorf("%w: %s; no written Item can repair it", ErrScenarioGreen, scenario.ID)
		}
	}

	for item := range affected {
		item.Findings = []string{finding}
	}

	slog.Error("Scenario green without target", "scenarios", green, "items", len(affected), "finding", finding)

	return nil
}

func (e *buildStepsExecution) markGreenItems(scenario *registry.RegistryScenario, compiled *catalogue.Catalogue,
	affected map[*BuildStepsItem]bool) bool {
	found := false

	for _, item := range e.items {
		if item.Suite != stepcoverage.StepsDir(scenario.Path) {
			continue
		}

		if item.Concept != nil {
			if conceptItemUsed(item, scenario) {
				affected[item], found = true, true
			}

			continue
		}

		for _, step := range scenario.Phrases() {
			for _, phrase := range compiled.Match(step.Text) {
				if phrase.Template == item.Phrase.Template {
					affected[item] = true
					found = true
				}
			}
		}
	}

	return found
}

func nameRedResults(verdict *testrunner.RedRun, scenarios []*registry.RegistryScenario, identities []string) {
	for index, identity := range identities {
		if slices.Contains(verdict.Red, identity) {
			slog.Info("Scenario red without target", "scenario", scenarios[index].ID, "test", identity)
		}

		for findingIndex, finding := range verdict.Invalid {
			if strings.HasPrefix(finding, identity+":") {
				prefix := scenarios[index].ID + " (" + identity + "):"
				verdict.Invalid[findingIndex] = prefix + strings.TrimPrefix(finding, identity+":")
			}
		}
	}
}
