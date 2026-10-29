// Package product reads the product document as data: the step-phrase
// catalogue, the roles its templates may name, and the glossary defining
// the terms its entries declare. Everything else in the document still
// reaches the checklists only as prompt text.
package product

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// ErrCatalogueNotList signals a `catalogue:` key holding something other
// than a list: the document meant to declare one, so it is not read as absent.
var ErrCatalogueNotList = errors.New("catalogue: must hold a list of phrases")

// Catalogue is what the closure checks read of the product document.
// Declared is false when the document, or its `catalogue:` key, is absent:
// a host that never wrote one has its Scenarios left unchecked.
type Catalogue struct {
	ConceptsDeclared bool
	Concepts         []Concept
	Declared         bool
	Entries          []catalogue.Entry
	Roles            []string
}

// LoadCatalogue reads the catalogue and the roles from the product
// document at path. An empty path or a missing file is not declared.
func LoadCatalogue(path string) (Catalogue, error) {
	doc, found, err := readDocument(path)
	if err != nil || !found {
		return Catalogue{}, err
	}

	roles := make([]string, 0, len(doc.Roles))
	for _, role := range doc.Roles {
		roles = append(roles, role.Name)
	}

	concepts, err := decodeConcepts(path, &doc.Concepts)
	if err != nil {
		return Catalogue{}, err
	}

	if doc.Concepts.Kind != 0 {
		return Catalogue{ConceptsDeclared: true, Concepts: concepts, Roles: roles}, nil
	}

	return decodeCatalogue(path, &doc.Catalogue, roles)
}

// decodeCatalogue reads the `catalogue:` node: absent or null is not
// declared, an empty list is, and any other shape is refused.
func decodeCatalogue(path string, node *yaml.Node, roles []string) (Catalogue, error) {
	if docs.Absent(node) {
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
