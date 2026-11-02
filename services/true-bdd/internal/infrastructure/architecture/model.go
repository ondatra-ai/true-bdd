package architecture

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/promptmodel"
)

// Model declares the YAML input of a service's template pair.
type Model struct {
	Templates ModelTemplates `yaml:"templates"`
	Scheme    string         `yaml:"scheme"`
}

// ModelTemplates names the two files filled from the same Model.
type ModelTemplates struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// ErrModelDeclaration distinguishes an invalid declaration or missing Scenario reference.
var ErrModelDeclaration = errors.New("invalid Model declaration")

// CheckModels holds declared resources and Model names to the complete Registry's service steps.
// An absent architecture declares no Models; malformed documents remain errors.
func CheckModels(path, root string, serviceSteps map[string][]string) error {
	var document struct {
		Architecture struct {
			Services []Service `yaml:"services"`
		} `yaml:"architecture"`
	}

	_, err := docs.ReadYAML(path, "architecture document", &document)
	if err != nil {
		return fmt.Errorf("check Models: %w", err)
	}

	for _, service := range document.Architecture.Services {
		for _, name := range slices.Sorted(maps.Keys(service.Models)) {
			err = service.Models[name].check(name, root, serviceSteps[service.Name])
			if err != nil {
				return fmt.Errorf("%w: service %s Model %s: %w", ErrModelDeclaration, service.Name, name, err)
			}
		}
	}

	return nil
}

func (m Model) check(name, root string, steps []string) error {
	for _, path := range []string{m.Templates.System, m.Templates.User, m.Scheme} {
		if path == "" {
			return fmt.Errorf("scheme and both templates must name files: %w", ErrModelDeclaration)
		}

		_, err := disk.Read(modelPath(root, path))
		if err != nil {
			return fmt.Errorf("resource %s: %w", path, err)
		}
	}

	_, err := promptmodel.Load(name, modelPath(root, m.Scheme))
	if err != nil {
		return fmt.Errorf("scheme: %w", err)
	}

	for _, step := range steps {
		words := strings.FieldsFunc(step, func(character rune) bool {
			return !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_'
		})
		if slices.Contains(words, name) {
			return nil
		}
	}

	return fmt.Errorf("no Scenario step names this Model: %w", ErrModelDeclaration)
}

func modelPath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(root, path)
}
