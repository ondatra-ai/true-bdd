// Package product reads the product document as data: the concepts its
// steps link to and the glossary defining the terms those concepts
// declare. Everything else in the document still reaches the checklists
// only as prompt text.
package product

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// ErrCatalogueRetired refuses a product document still declaring the
// retired `catalogue:` of step templates, naming what replaced it.
var ErrCatalogueRetired = errors.New("catalogue: is no longer read; declare concepts: in its place (ADR-0034)")

// Document is what the engine reads of the product document. ConceptsDeclared
// is false when the document, or its `concepts:` key, is absent: a host that
// never wrote concepts has its Scenarios left unchecked.
type Document struct {
	ConceptsDeclared bool
	Concepts         []Concept
}

// rawDocument is the slice of the product document the checks decode.
type rawDocument struct {
	Concepts  yaml.Node `yaml:"concepts"`
	Catalogue yaml.Node `yaml:"catalogue"`
	Glossary  yaml.Node `yaml:"glossary"`
}

// Load reads the concepts of the product document at path. An empty path
// or a missing file declares none.
func Load(path string) (Document, error) {
	doc, found, err := readDocument(path)
	if err != nil || !found {
		return Document{}, err
	}

	concepts, err := decodeConcepts(path, &doc.Concepts)
	if err != nil {
		return Document{}, err
	}

	return Document{ConceptsDeclared: doc.Concepts.Kind != 0, Concepts: concepts}, nil
}

// readDocument parses the product document at path; found is false for
// an empty path or a missing file, which a host without one has.
func readDocument(path string) (*rawDocument, bool, error) {
	var doc rawDocument

	found, err := docs.ReadYAML(path, "product document", &doc)
	if err != nil || !found {
		return nil, false, err
	}

	if doc.Catalogue.Kind != 0 {
		return nil, false, fmt.Errorf("product document %s: %w", path, ErrCatalogueRetired)
	}

	return &doc, true, nil
}
