package product

import (
	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
)

// productDocument is the slice of the product document the checks decode.
type productDocument struct {
	Roles []struct {
		Name string `yaml:"name"`
	} `yaml:"roles"`
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

	return &doc, true, nil
}
