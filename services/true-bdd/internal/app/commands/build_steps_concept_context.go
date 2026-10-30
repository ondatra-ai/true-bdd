package commands

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

type conceptItemContext struct {
	Concept     conceptMeaning                `yaml:"concept"`
	Binding     architecture.ConceptBinding   `yaml:"binding"`
	Glossary    []catalogue.Definition        `yaml:"glossary"`
	Words       []catalogue.Mapping           `yaml:"word_mapping"`
	Interface   *architecture.PublicInterface `yaml:"public_interface"`
	Document    string                        `yaml:"interface_document"`
	Definitions []conceptDefinitionContext    `yaml:"definitions"`
	Scenarios   []stepScenarioContext         `yaml:"scenarios"`
	Fixtures    []conceptFixture              `yaml:"fixture_inputs,omitempty"`
}

type conceptMeaning struct {
	Concept string          `yaml:"concept"`
	Block   catalogue.Block `yaml:"block"`
	Means   string          `yaml:"means"`
	Terms   []string        `yaml:"terms"`
}

type conceptDefinitionContext struct {
	Function string                     `yaml:"function"`
	File     string                     `yaml:"file"`
	Pattern  string                     `yaml:"pattern"`
	Captures []catalogue.PatternCapture `yaml:"captures,omitempty"`
	Serves   string                     `yaml:"serves,omitempty"`
	Steps    []conceptBoundStep         `yaml:"steps"`
}

type conceptBoundStep struct {
	Text     string   `yaml:"text"`
	Captures []string `yaml:"captures"`
}

// conceptSpecification is what every concept Item's context reads from the Specification.
type conceptSpecification struct {
	concepts    []product.Concept
	definitions []catalogue.Definition
	mappings    []catalogue.Mapping
	bindings    []architecture.ConceptBinding
}

func describeConceptItem(item *BuildStepsItem, arch *architecture.Architecture, spec conceptSpecification,
	discovery *conceptDiscovery,
) error {
	context := spec.meaning(item)

	for _, entry := range spec.definitions {
		if slices.Contains(context.Concept.Terms, entry.Term) {
			context.Glossary = append(context.Glossary, entry)
		}
	}

	for _, entry := range spec.mappings {
		if slices.Contains(context.Concept.Terms, entry.Term) {
			context.Words = append(context.Words, entry)
		}
	}

	err := conceptInterfaceDocument(&context, arch, item.Service)
	if err != nil {
		return err
	}

	context.Definitions = conceptDefinitionContexts(item)
	context.Scenarios = conceptScenarios(item, discovery)

	context.Fixtures, err = conceptFixtures(item, discovery.scenarios)
	if err != nil {
		return err
	}

	rendered, err := yaml.Marshal(context)
	if err != nil {
		return fmt.Errorf("render concept context: %w", err)
	}

	item.Context = string(rendered)

	return nil
}

// meaning is the concept's own entry and its binding for the Item's service.
func (spec conceptSpecification) meaning(item *BuildStepsItem) conceptItemContext {
	context := conceptItemContext{}

	for _, concept := range spec.concepts {
		if concept.Name == item.Concept.Name && concept.Block == item.Concept.Block {
			item.Concept.Means = concept.Means
			context.Concept = conceptMeaning{concept.Name, concept.Block, concept.Means, concept.Terms}
		}
	}

	for _, binding := range spec.bindings {
		if binding.Concept == item.Concept.Name && binding.Block == item.Concept.Block && binding.Service == item.Service {
			context.Binding = binding
		}
	}

	return context
}

func conceptInterfaceDocument(context *conceptItemContext, arch *architecture.Architecture, service string) error {
	for _, candidate := range arch.Services {
		if candidate.Name != service || candidate.PublicInterface == nil {
			continue
		}

		context.Interface = candidate.PublicInterface
		if candidate.PublicInterface.Document == "" {
			continue
		}

		data, err := disk.Read(candidate.PublicInterface.Document)
		if err != nil {
			return fmt.Errorf("read Public interface: %w", err)
		}

		context.Document = string(data)
	}

	return nil
}

func conceptDefinitionContexts(item *BuildStepsItem) []conceptDefinitionContext {
	contexts := make([]conceptDefinitionContext, 0, len(item.Concept.Definitions))

	for _, definition := range item.Concept.Definitions {
		context := conceptDefinitionContext{Function: definition.Function, File: definition.File,
			Pattern: definition.Pattern}

		for _, pattern := range item.Concept.Patterns {
			if pattern.Pattern == definition.Pattern {
				context.Captures, context.Serves = pattern.Captures, pattern.Serves
			}
		}

		for _, step := range item.Concept.Steps {
			if captures := catalogue.PatternCaptures(definition.Pattern, step); captures != nil {
				context.Steps = append(context.Steps, conceptBoundStep{Text: step, Captures: captures})
			}
		}

		contexts = append(contexts, context)
	}

	return contexts
}

func conceptScenarios(item *BuildStepsItem, discovery *conceptDiscovery) []stepScenarioContext {
	using := make([]stepScenarioContext, 0)

	for _, scenario := range discovery.scenarios {
		if stepcoverage.StepsDir(scenario.Path) != item.Suite {
			continue
		}

		for _, step := range scenario.Phrases() {
			if step.Block == item.Concept.Block && slices.Contains(item.Concept.Steps, step.Text) {
				using = append(using, stepScenarioContext{scenario.ID, scenario.Description,
					scenario.Service, scenario.Path, scenario.FormatSteps()})

				break
			}
		}
	}

	return using
}
