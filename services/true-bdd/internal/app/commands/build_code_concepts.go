package commands

import (
	"log/slog"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// codeConceptSpecifications reads, per Scenario, the concepts its steps link
// to: the concept recorded on the one definition each deterministic step
// binds, with its binding for the Scenario's service. Unreadable parts warn.
func codeConceptSpecifications(source product.Document, productFile, architectureFile string,
	scenarios []*registry.RegistryScenario,
) map[string]catalogue.Specification {
	specs := make(map[string]catalogue.Specification)

	definitions, mappings, bindings, ok := codeConceptDocuments(productFile, architectureFile)
	if !ok {
		return specs
	}

	suites := make(map[string]*codeConceptSuite)

	for _, scenario := range scenarios {
		dir := stepcoverage.StepsDir(scenario.Path)
		if suites[dir] == nil {
			suites[dir] = loadCodeConceptSuite(dir)
		}

		linked := suites[dir].linked(scenario, source.Concepts, bindings)
		specs[scenario.ID] = catalogue.SpecifyConcepts(linked, definitions, mappings)
	}

	return specs
}

func codeConceptDocuments(productFile, architectureFile string) (
	[]catalogue.Definition, []catalogue.Mapping, []architecture.ConceptBinding, bool,
) {
	definitions, err := product.LoadGlossary(productFile)
	if err != nil {
		slog.Warn("Cannot read optional Specification document", "document", productFile, "error", err)

		return nil, nil, nil, false
	}

	mappings, err := architecture.LoadTermMappings(architectureFile)
	if err != nil {
		slog.Warn("Cannot read optional Specification document", "document", architectureFile, "error", err)

		return nil, nil, nil, false
	}

	bindings, err := architecture.LoadConceptBindings(architectureFile)
	if err != nil {
		slog.Warn("Cannot read optional Specification document", "document", architectureFile, "error", err)

		return nil, nil, nil, false
	}

	return definitions, mappings, bindings, true
}

// codeConceptSuite is a suite's definitions and the concepts they record;
// nil fields mean the suite could not be read and links no concept.
type codeConceptSuite struct {
	suite    *stepcoverage.Suite
	recorded []stepskeleton.ConceptDefinition
}

func loadCodeConceptSuite(dir string) *codeConceptSuite {
	suite, err := stepcoverage.LoadSuite(dir)
	if err != nil {
		slog.Warn("Cannot read step definitions", "suite", dir, "error", err)

		return &codeConceptSuite{}
	}

	recorded, err := stepskeleton.Concepts(dir)
	if err != nil {
		slog.Warn("Cannot read step definitions", "suite", dir, "error", err)

		return &codeConceptSuite{}
	}

	return &codeConceptSuite{suite: suite, recorded: recorded}
}

// linked lists each concept the Scenario's steps link to once, in step order.
func (s *codeConceptSuite) linked(scenario *registry.RegistryScenario, concepts []product.Concept,
	bindings []architecture.ConceptBinding,
) []catalogue.ConceptSpec {
	linked := make([]catalogue.ConceptSpec, 0)
	if s.suite == nil {
		return linked
	}

	for _, step := range scenario.CheckedSteps() {
		concept, found := s.concept(step, concepts)
		if !found || slices.ContainsFunc(linked, func(spec catalogue.ConceptSpec) bool {
			return spec.Concept == concept.Name && spec.Block == concept.Block
		}) {
			continue
		}

		linked = append(linked, catalogue.ConceptSpec{Concept: concept.Name, Block: concept.Block,
			Means: concept.Means, Terms: append(make([]string, 0, len(concept.Terms)), concept.Terms...),
			Bindings: conceptUses(concept, scenario.Service, bindings)})
	}

	return linked
}

// concept is the declared concept of the step's block that the one definition it binds records.
func (s *codeConceptSuite) concept(step catalogue.Step, concepts []product.Concept) (product.Concept, bool) {
	if step.ModelRun() {
		return product.Concept{}, false
	}

	bound := s.suite.Bind(step.Text)
	if len(bound) != 1 {
		return product.Concept{}, false
	}

	for _, record := range s.recorded {
		if record.Pattern != bound[0].Pattern || record.Block != string(step.Block) {
			continue
		}

		for _, concept := range concepts {
			if concept.Block == step.Block && concept.Name == record.Concept {
				return concept, true
			}
		}
	}

	return product.Concept{}, false
}

func conceptUses(concept product.Concept, service string, bindings []architecture.ConceptBinding,
) []catalogue.ConceptUse {
	uses := make([]catalogue.ConceptUse, 0, 1)

	for _, binding := range bindings {
		if binding.Concept != concept.Name || binding.Block != concept.Block || binding.Service != service {
			continue
		}

		use := catalogue.ConceptUse{Service: binding.Service, Does: binding.Does, Intent: binding.Intent}
		for _, reference := range binding.References {
			use.References = append(use.References, reference.Selector())
		}

		uses = append(uses, use)
	}

	return uses
}
