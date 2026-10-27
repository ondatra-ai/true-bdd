package commands

import (
	"errors"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// specifyCodeFailures enriches discovered tests without adding a startup
// precondition: optional Specification documents warn and are left out.
func specifyCodeFailures(
	deps BuildCodeDeps, arch *architecture.Architecture, architectureFile string,
	failures []*testrunner.FailingTest,
) {
	scenarios := codeRegistry(deps.DocResolver)
	specs := codeStepSpecifications(deps.DocResolver, architectureFile, scenarios)
	identities := codeScenarioIdentities(scenarios)

	for _, failure := range failures {
		if len(arch.Services) == 1 {
			failure.Service = arch.Services[0].Name
		}

		if failure.Framework != scenariogen.FrameworkGoTest {
			continue
		}

		if scenario := identities[failure.TestName]; scenario != nil {
			failure.Scenario = scenario
			failure.Service = scenario.Service
			failure.Spec = specs[scenario.ID]
		}
	}
}

func optionalCodeDocument(resolver *docs.Resolver, key string) string {
	file, err := resolver.Resolve(key)
	if err != nil && !errors.Is(err, docs.ErrDocPathNotConfigured) {
		slog.Warn("Cannot read optional Specification document", "document", key, "error", err)
	}

	return file
}

func codeRegistry(resolver *docs.Resolver) []*registry.RegistryScenario {
	file := optionalCodeDocument(resolver, docs.KeyScenariosYAML)
	if file == "" {
		return nil
	}

	scenarios, err := registry.NewRegistryLoader().Load(file)
	if err != nil {
		slog.Warn("Cannot read optional Specification document", "document", file, "error", err)
	}

	return scenarios
}

func codeStepSpecifications(
	resolver *docs.Resolver, architectureFile string, scenarios []*registry.RegistryScenario,
) map[string]catalogue.Specification {
	specs := make(map[string]catalogue.Specification)

	file := optionalCodeDocument(resolver, docs.KeyProduct)
	if file == "" {
		return specs
	}

	source, err := product.LoadCatalogue(file)
	if err != nil {
		slog.Warn("Cannot read optional Specification document", "document", file, "error", err)

		return specs
	}

	if !source.Declared {
		return specs
	}

	compiled, definitions, mappings, err := compileStepSpecification(source, file, architectureFile)
	if err != nil {
		slog.Warn("Cannot read optional Specification document", "document", file, "error", err)

		return specs
	}

	for _, scenario := range scenarios {
		specs[scenario.ID] = compiled.Specify(scenario.Phrases(), nil, definitions, mappings)
	}

	return specs
}

// codeScenarioIdentities resolves each Scenario once per discovery, rather
// than re-reading its go.mod for every failing test in the walk.
func codeScenarioIdentities(scenarios []*registry.RegistryScenario) map[string]*registry.RegistryScenario {
	identities := make(map[string]*registry.RegistryScenario, len(scenarios))
	for _, scenario := range scenarios {
		identity, err := scenariogen.TestIdentity(".", scenario)
		if err != nil {
			slog.Warn("Cannot resolve Scenario test package", "path", scenario.Path, "error", err)

			continue
		}

		if identities[identity] == nil {
			identities[identity] = scenario
		}
	}

	return identities
}
