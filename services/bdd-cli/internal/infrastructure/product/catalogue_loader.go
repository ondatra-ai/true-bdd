// Package product reads the product document as data: the step-phrase
// catalogue and the roles its templates may name. Everything else in the
// document still reaches the checklists only as prompt text.
package product

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
)

// ErrCatalogueNotList signals a `catalogue:` key holding something other
// than a list: the document meant to declare one, so it is not read as absent.
var ErrCatalogueNotList = errors.New("catalogue: must hold a list of phrases")

// Catalogue is what the closure checks read of the product document.
// Declared is false when the document, or its `catalogue:` key, is absent:
// a host that never wrote one has its Scenarios left unchecked.
type Catalogue struct {
	Declared bool
	Entries  []catalogue.Entry
	Roles    []string
}

// productDocument is the slice of the product document the checks decode.
type productDocument struct {
	Roles []struct {
		Name string `yaml:"name"`
	} `yaml:"roles"`
	Catalogue yaml.Node `yaml:"catalogue"`
}

// LoadCatalogue reads the catalogue and the roles from the product
// document at path. An empty path or a missing file is not declared.
func LoadCatalogue(path string) (Catalogue, error) {
	if path == "" {
		return Catalogue{}, nil
	}

	data, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Catalogue{}, nil
	}

	if err != nil {
		return Catalogue{}, fmt.Errorf("read product document %s: %w", path, err)
	}

	var doc productDocument

	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		return Catalogue{}, fmt.Errorf("parse product document %s: %w", path, err)
	}

	roles := make([]string, 0, len(doc.Roles))
	for _, role := range doc.Roles {
		roles = append(roles, role.Name)
	}

	return decodeCatalogue(path, &doc.Catalogue, roles)
}

// decodeCatalogue reads the `catalogue:` node: absent or null is not
// declared, an empty list is, and any other shape is refused.
func decodeCatalogue(path string, node *yaml.Node, roles []string) (Catalogue, error) {
	if node.Kind == 0 || node.Tag == "!!null" {
		return Catalogue{Roles: roles}, nil
	}

	if node.Kind != yaml.SequenceNode {
		return Catalogue{}, fmt.Errorf("product document %s: %w", path, ErrCatalogueNotList)
	}

	entries := make([]catalogue.Entry, 0, len(node.Content))

	err := node.Decode(&entries)
	if err != nil {
		return Catalogue{}, fmt.Errorf("product document %s: catalogue: %w", path, err)
	}

	return Catalogue{Declared: true, Entries: entries, Roles: roles}, nil
}
