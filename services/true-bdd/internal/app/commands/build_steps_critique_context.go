package commands

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
	"gopkg.in/yaml.v3"
)

// critiqueContext supplies complete referenced evidence units, with no fixture files or provenance corpus.
func critiqueContext(item *BuildStepsItem, sources string) (string, error) {
	var evidence conceptItemContext

	err := yaml.Unmarshal([]byte(item.Context), &evidence)
	if err != nil {
		return "", fmt.Errorf("read verification context: %w", err)
	}

	evidence.Registry, evidence.FixtureRoot = "", ""
	evidence.Scenarios, evidence.Fixtures = nil, nil

	selected, err := selectedRuntimeAPI(evidence.RuntimeAPI, sources)
	if err != nil {
		return "", err
	}

	evidence.RuntimeAPI = selected

	origin := ""
	if evidence.Interface != nil {
		origin = evidence.Interface.Document
	}

	document, err := verification.InterfaceEntries(evidence.Document, evidence.Binding.References, origin)
	if err != nil {
		return "", err
	}

	evidence.Document = document
	for index := range evidence.Definitions {
		definition := &evidence.Definitions[index]

		texts := make([]string, 0, len(definition.Steps))
		for _, step := range definition.Steps {
			texts = append(texts, step.Text)
		}

		definition.Forms, err = verification.Forms(definition.Pattern, texts)
		if err != nil {
			return "", err
		}

		definition.Steps = nil
	}

	rendered, err := yaml.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("render verification context: %w", err)
	}

	return string(rendered), nil
}

func selectedRuntimeAPI(paths map[string]string, sources string) (map[string]string, error) {
	selected := make(map[string]string)

	for imported, path := range paths {
		documentation, err := disk.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read Go documentation for %s: %w", imported, err)
		}

		if declarations := verification.RuntimeDocumentation(string(documentation), sources); declarations != "" {
			selected[imported] = declarations
		}
	}

	return selected, nil
}
