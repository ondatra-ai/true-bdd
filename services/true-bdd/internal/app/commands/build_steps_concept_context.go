package commands

import (
	"fmt"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
)

type conceptItemContext struct {
	Concept     conceptMeaning                `yaml:"concept"`
	Binding     architecture.ConceptBinding   `yaml:"binding"`
	Glossary    []catalogue.Definition        `yaml:"glossary"`
	Words       []catalogue.Mapping           `yaml:"word_mapping"`
	Interface   *architecture.PublicInterface `yaml:"public_interface"`
	Document    string                        `yaml:"interface_document"`
	RuntimeAPI  map[string]string             `yaml:"runtime_api,omitempty"`
	Definitions []conceptDefinitionContext    `yaml:"definitions"`
	Registry    string                        `yaml:"registry,omitempty"`
	Scenarios   []string                      `yaml:"scenarios,omitempty"`
	FixtureRoot string                        `yaml:"fixture_root,omitempty"`
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
	Steps    []conceptBoundStep         `yaml:"steps,omitempty"`
	Forms    []verification.Form        `yaml:"forms,omitempty"`
}

type conceptBoundStep struct {
	Text     string   `yaml:"text"`
	Captures []string `yaml:"captures"`
}

type stepScenarioContext struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Service     string `yaml:"service"`
	Path        string `yaml:"path"`
	Steps       string `yaml:"steps"`
}

func describeConceptItem(item *BuildStepsItem, arch *architecture.Architecture, spec conceptSpecification,
	discovery *conceptDiscovery, runtimeAPI ...map[string]string,
) error {
	context := spec.meaning(item)
	if len(runtimeAPI) > 0 {
		context.RuntimeAPI = runtimeAPI[0]
	}

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

	context.Registry = discovery.registryFile
	for _, scenario := range conceptScenarios(item, discovery) {
		context.Scenarios = append(context.Scenarios, scenario.ID)
	}

	context.Fixtures = conceptFixtures(item, discovery.scenarios)
	if len(context.Fixtures) > 0 {
		context.FixtureRoot = filepath.ToSlash(filepath.Join(filepath.Dir(item.Suite), "fixtures"))
	}

	rendered, err := yaml.Marshal(context)
	if err != nil {
		return fmt.Errorf("render concept context: %w", err)
	}

	item.Context = string(rendered)

	return nil
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

		for _, step := range scenario.CheckedSteps() {
			if step.Block == item.Concept.Block && slices.Contains(item.Concept.Steps, step.Text) {
				using = append(using, stepScenarioContext{scenario.ID, scenario.Description,
					scenario.Service, scenario.Path, scenario.FormatSteps()})

				break
			}
		}
	}

	return using
}
