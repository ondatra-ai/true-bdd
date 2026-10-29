package architecture

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"gopkg.in/yaml.v3"
)

// InterfaceDocument is one document a service's Public interface declaration
// names, its path relative to the Host project root as declared.
type InterfaceDocument struct {
	Service string
	Kind    string
	Path    string
}

// DeclaredInterfaceDocuments lists the cli and http documents the
// architecture's declarations name, in service order, each path once.
func DeclaredInterfaceDocuments(path string) ([]InterfaceDocument, error) {
	data, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read architecture file %s: %w", path, err)
	}

	var raw struct {
		Architecture struct {
			Services []Service `yaml:"services"`
		} `yaml:"architecture"`
	}

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse architecture file %s: %w", path, err)
	}

	documents := make([]InterfaceDocument, 0, len(raw.Architecture.Services))
	seen := make(map[string]bool)

	for _, service := range raw.Architecture.Services {
		declaration := service.PublicInterface
		if declaration == nil || declaration.Document == "" || seen[declaration.Document] ||
			(declaration.Kind != kindCLI && declaration.Kind != kindHTTP) {
			continue
		}

		seen[declaration.Document] = true
		documents = append(documents, InterfaceDocument{
			Service: service.Name, Kind: declaration.Kind, Path: declaration.Document,
		})
	}

	return documents, nil
}

// CheckInterfaceDocument parses a document's bytes in the form of its kind;
// path places an OpenAPI document's local references, root the Host project.
func CheckInterfaceDocument(kind string, data []byte, path, root string) error {
	return parseInterfaceDocument(&PublicInterface{Kind: kind}, data, path, root)
}
