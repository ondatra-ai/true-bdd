package commands

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

type stepPhraseContext struct {
	Phrase    catalogue.Entry               `yaml:"phrase"`
	Mapping   architecture.PhraseMapping    `yaml:"phrase_mapping"`
	Glossary  []catalogue.Definition        `yaml:"glossary"`
	Words     []catalogue.Mapping           `yaml:"word_mapping"`
	Interface *architecture.PublicInterface `yaml:"public_interface"`
	Document  string                        `yaml:"interface_document"`
	Scenarios []stepScenarioContext         `yaml:"scenarios"`
}

type stepScenarioContext struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Service     string `yaml:"service"`
	Path        string `yaml:"path"`
	Steps       string `yaml:"steps"`
}

func describeStepItem(item *BuildStepsItem, arch *architecture.Architecture, definitions []catalogue.Definition,
	mappings []catalogue.Mapping, scenarios []*registry.RegistryScenario, compiled *catalogue.Catalogue) error {
	service, mapping, err := stepItemMapping(item, arch)
	if err != nil {
		return err
	}

	context := stepPhraseContext{Phrase: item.Phrase, Mapping: mapping, Interface: service.PublicInterface}
	for _, entry := range definitions {
		if slices.Contains(item.Phrase.Terms, entry.Term) {
			context.Glossary = append(context.Glossary, entry)
		}
	}

	for _, entry := range mappings {
		if slices.Contains(item.Phrase.Terms, entry.Term) {
			context.Words = append(context.Words, entry)
		}
	}

	if service.PublicInterface.Document != "" {
		data, err := disk.Read(service.PublicInterface.Document)
		if err != nil {
			return fmt.Errorf("read Public interface: %w", err)
		}

		context.Document = string(data)
	}

	context.Scenarios = usingStepScenarios(item, scenarios, compiled)

	rendered, err := yaml.Marshal(context)
	if err != nil {
		return fmt.Errorf("render step context: %w", err)
	}

	item.Context = string(rendered)

	return nil
}

func stepItemMapping(item *BuildStepsItem, arch *architecture.Architecture) (
	architecture.Service, architecture.PhraseMapping, error,
) {
	var service architecture.Service

	for _, candidate := range arch.Services {
		if candidate.Name == item.Service {
			service = candidate
		}
	}

	if finding := service.PublicInterface.Finding(); finding != "" {
		return service, architecture.PhraseMapping{}, fmt.Errorf("%w: %s: %s", ErrStepSpecification, item.Service, finding)
	}

	var entries []architecture.PhraseMapping

	for _, entry := range arch.Phrases {
		if entry.Service == item.Service && entry.Template == item.Phrase.Template {
			entries = append(entries, entry)
		}
	}

	if len(entries) != 1 {
		return service, architecture.PhraseMapping{}, fmt.Errorf("%w: phrase %q needs one Phrase mapping for service %s",
			ErrStepSpecification, item.Phrase.Template, item.Service)
	}

	return service, entries[0], nil
}

func usingStepScenarios(item *BuildStepsItem, scenarios []*registry.RegistryScenario,
	compiled *catalogue.Catalogue) []stepScenarioContext {
	using := make([]stepScenarioContext, 0)

	for _, scenario := range scenarios {
		if stepcoverage.StepsDir(scenario.Path) != item.Suite {
			continue
		}

		for _, step := range scenario.Phrases() {
			matched := compiled.Match(step.Text)
			if len(matched) == 1 && matched[0].Template == item.Phrase.Template {
				using = append(using, stepScenarioContext{scenario.ID, scenario.Description,
					scenario.Service, scenario.Path, scenario.FormatSteps()})

				break
			}
		}
	}

	return using
}
