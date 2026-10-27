package commands

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
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
// by Scenario id: only under --fix, where a step-authoring turn reads it,
// and only from a declared catalogue, source, every step already matched.
func (p *buildTestsPrep) specify(
	source product.Catalogue,
	scenarios []*registry.RegistryScenario,
	answer *stepcoverage.Answer,
) (map[string]catalogue.Specification, error) {
	specs := make(map[string]catalogue.Specification, len(answer.Gaps))
	if !p.fix || len(answer.Gaps) == 0 || !source.Declared {
		return specs, nil
	}

	compiled, definitions, mappings, err := compileStepSpecification(source, p.productFile, p.architectureFile)
	if err != nil {
		return nil, err
	}

	sweeps := unboundBySuite(scenarios, answer)

	for _, scenario := range scenarios {
		own := answer.Gaps[scenario.ID]
		if len(own) == 0 {
			continue
		}

		spec := compiled.Specify(own, sweeps[stepcoverage.StepsDir(scenario.Path)], definitions, mappings)
		specs[scenario.ID] = spec

		slog.Info("Unbound steps specified", "subjectID", scenario.ID,
			"steps", len(spec.Steps), "phrases", len(spec.Phrases), "terms", len(spec.Terms))
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

// compileStepSpecification reads the glossary and word mapping shared by
// build tests and build code. Both turns use Catalogue.Specify unchanged.
func compileStepSpecification(source product.Catalogue, productFile, architectureFile string) (
	*catalogue.Catalogue, []catalogue.Definition, []catalogue.Mapping, error,
) {
	definitions, err := product.LoadGlossary(productFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read the glossary: %w", err)
	}

	mappings, err := architecture.LoadTermMappings(architectureFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read the word mapping: %w", err)
	}

	compiled, _ := catalogue.Compile(source.Entries, source.Roles)

	return compiled, definitions, mappings, nil
}
