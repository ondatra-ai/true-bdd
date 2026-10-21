package commands

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/stepcoverage"
)

// productDocument is the product document's path, or "" for a host that
// has none, which authors its steps as before: there is no catalogue to read.
func productDocument(resolver *docs.Resolver) string {
	path, err := resolver.Resolve(docs.KeyProduct)
	if err != nil {
		return ""
	}

	return path
}

// specify reads what the spec says about each Scenario's unbound steps,
// by Scenario id. Only under --fix, since only a step-authoring turn
// reads it, and only from a product document declaring a catalogue.
func (p *buildTestsPrep) specify(
	scenarios []*registry.RegistryScenario,
	answer *stepcoverage.Answer,
) (map[string]catalogue.Specification, error) {
	specs := make(map[string]catalogue.Specification, len(answer.Gaps))
	if !p.fix || len(answer.Gaps) == 0 {
		return specs, nil
	}

	source, err := product.LoadCatalogue(p.productFile)
	if err != nil {
		return nil, fmt.Errorf("read the catalogue: %w", err)
	}

	if !source.Declared {
		return specs, nil
	}

	definitions, err := product.LoadGlossary(p.productFile)
	if err != nil {
		return nil, fmt.Errorf("read the glossary: %w", err)
	}

	mappings, err := architecture.LoadTermMappings(p.architectureFile)
	if err != nil {
		return nil, fmt.Errorf("read the word mapping: %w", err)
	}

	// A template the grammar refuses phrases nothing here; the
	// step-phrases check is what reports it.
	compiled, _ := catalogue.Compile(source.Entries, source.Roles)
	sweeps := unboundBySuite(scenarios, answer)

	for _, scenario := range scenarios {
		own := answer.Gaps[scenario.ID]
		if len(own) == 0 {
			continue
		}

		spec := compiled.Specify(own, sweeps[stepcoverage.StepsDir(scenario.Path)], definitions, mappings)
		specs[scenario.ID] = spec

		slog.Info("Unbound steps specified", "subjectID", scenario.ID,
			"steps", len(spec.Steps), "phrases", len(spec.Phrases), "terms", len(spec.Words))
	}

	return specs, nil
}

// unboundBySuite gathers the unbound steps of every Scenario, per suite
// and in Registry order: the steps a fix sweeps in beside its own.
func unboundBySuite(
	scenarios []*registry.RegistryScenario,
	answer *stepcoverage.Answer,
) map[string][]catalogue.Step {
	sweeps := make(map[string][]catalogue.Step)

	for _, scenario := range scenarios {
		dir := stepcoverage.StepsDir(scenario.Path)
		sweeps[dir] = append(sweeps[dir], answer.Gaps[scenario.ID]...)
	}

	return sweeps
}
