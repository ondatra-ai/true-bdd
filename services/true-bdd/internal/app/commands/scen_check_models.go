package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func checkScenarioModels(deps ScenCheckDeps, scenarios []*registry.RegistryScenario) error {
	path, err := deps.DocResolver.Resolve(docs.KeyArchitectureYAML)
	if err != nil {
		// Existing document checks own missing/unconfigured architecture findings.
		return nil //nolint:nilerr // No readable declaration can name a Model here.
	}

	steps := make(map[string][]string)

	for _, scenario := range scenarios {
		for _, step := range scenario.CheckedSteps() {
			steps[scenario.Service] = append(steps[scenario.Service], step.Text)
		}
	}

	err = architecture.CheckModels(path, ".", steps)
	if err != nil {
		return runner.RefuseStartup("scen check", err)
	}

	return nil
}
