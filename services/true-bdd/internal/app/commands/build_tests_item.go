package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

// BuildTestsItem is one Scenario as `build tests` walks it: its Registry
// entry, whose fields and FormatSteps the prompt templates read through
// it, and what the spec says about the steps it leaves unbound.
type BuildTestsItem struct {
	*registry.RegistryScenario

	// Spec is read under --fix for a Scenario with an unbound step, from
	// a product document declaring a catalogue; the zero value renders nothing.
	Spec catalogue.Specification

	// Failures carries runtime evidence into the step-authoring prompt.
	Failures []string
}

// buildTestsItems wraps each loaded Registry entry, in the loader's order.
func buildTestsItems(scenarios []*registry.RegistryScenario) []*BuildTestsItem {
	items := make([]*BuildTestsItem, 0, len(scenarios))
	for _, scenario := range scenarios {
		items = append(items, &BuildTestsItem{RegistryScenario: scenario})
	}

	return items
}

// registryScenarios unwraps the items for the code reading the Registry
// alone: the test renderer and the step-coverage scan.
func registryScenarios(items []*BuildTestsItem) []*registry.RegistryScenario {
	scenarios := make([]*registry.RegistryScenario, 0, len(items))
	for _, item := range items {
		scenarios = append(scenarios, item.RegistryScenario)
	}

	return scenarios
}

// buildTestsSubject is the GetSubject implementation: the Scenario's id
// and description, as for every Registry walk.
func buildTestsSubject(item *BuildTestsItem) (string, string) {
	return registry.Subject(item.RegistryScenario)
}
