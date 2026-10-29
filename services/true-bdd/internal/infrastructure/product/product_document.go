package product

import (
	"fmt"
	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// productDocument is the slice of the product document the checks decode.
type productDocument struct {
	Roles []struct {
		Name string `yaml:"name"`
	} `yaml:"roles"`
	Concepts  yaml.Node `yaml:"concepts"`
	Catalogue yaml.Node `yaml:"catalogue"`
	Glossary  yaml.Node `yaml:"glossary"`
}

// readDocument parses the product document at path; found is false for
// an empty path or a missing file, which a host without one has.
func readDocument(path string) (*productDocument, bool, error) {
	var doc productDocument

	found, err := docs.ReadYAML(path, "product document", &doc)
	if err != nil || !found {
		return nil, false, err
	}

	if doc.Catalogue.Kind != 0 && doc.Concepts.Kind != 0 {
		return nil, false, fmt.Errorf("product document %s: %w", path, ErrBothForms)
	}

	return &doc, true, nil
}
