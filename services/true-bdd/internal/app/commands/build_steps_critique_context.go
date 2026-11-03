package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// critiqueContext embeds the semantic core and fixture contracts used by source or bound steps.
func critiqueContext(item *BuildStepsItem, sources string) (string, error) {
	var evidence conceptItemContext

	err := yaml.Unmarshal([]byte(item.Context), &evidence)
	if err != nil {
		return "", fmt.Errorf("read critique context: %w", err)
	}

	evidence.Registry = ""
	for index := range evidence.Fixtures {
		fixture := &evidence.Fixtures[index]

		files, err := fixtureInputs(filepath.Join(evidence.FixtureRoot, fixture.Fixture))
		if err != nil {
			return "", err
		}

		for _, file := range files {
			if usedFixtureContract(file.Path, sources, evidence.Definitions) {
				fixture.Files = append(fixture.Files, file)
			}
		}
	}

	rendered, err := yaml.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("render critique context: %w", err)
	}

	return string(rendered), nil
}

func usedFixtureContract(path, sources string, definitions []conceptDefinitionContext) bool {
	name := filepath.Base(path)
	switch name {
	case "prep.sh", "teardown.sh", "checklist-prompts.yaml":
		return true
	}

	if strings.Contains(sources, name) {
		return true
	}

	for _, definition := range definitions {
		for _, step := range definition.Steps {
			for _, capture := range step.Captures {
				if capture != "" && (strings.HasSuffix(path, "/"+capture) || name == capture) {
					return true
				}
			}
		}
	}

	return false
}
