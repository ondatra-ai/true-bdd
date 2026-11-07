package steps

import (
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
	"gopkg.in/yaml.v3"
)

func readValidationChecklist(state *State) ([]byte, error) {
	path, err := state.containedPath("true-bdd/true-bdd.yaml")
	if err != nil {
		return nil, err
	}

	content, err := testkit.Read(path)
	if err != nil {
		return nil, state.fail("read checklist configuration: %w", err)
	}

	var config struct {
		Paths struct {
			ChecklistsDir string `yaml:"checklists_dir"`
		} `yaml:"paths"`
	}

	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, state.fail("decode checklist configuration: %w", err)
	}

	path, err = state.containedPath(filepath.Join(config.Paths.ChecklistsDir, "scen-check.yaml"))
	if err != nil {
		return nil, err
	}

	content, err = testkit.Read(path)
	if err != nil {
		return nil, state.fail("read validation checklist: %w", err)
	}

	return content, nil
}

func validationChecklistSize(state *State) (int, error) {
	content, err := readValidationChecklist(state)
	if err != nil {
		return 0, err
	}

	var checklist struct {
		Sections []struct {
			Prompts []struct {
				Skip bool `yaml:"skip"`
			} `yaml:"validation_prompts"`
		} `yaml:"sections"`
	}

	err = yaml.Unmarshal(content, &checklist)
	if err != nil {
		return 0, state.fail("decode validation checklist: %w", err)
	}

	count := 0

	for _, section := range checklist.Sections {
		for _, prompt := range section.Prompts {
			if !prompt.Skip {
				count++
			}
		}
	}

	if count == 0 {
		return 0, state.fail("validation checklist has no items")
	}

	return count, nil
}
