package product

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// ErrGlossaryNotList signals a `glossary:` key holding something other
// than a list, which no term could be looked up in.
var ErrGlossaryNotList = errors.New("glossary: must hold a list of terms")

// LoadGlossary reads every definition of the product glossary at path, a
// term defined twice listed twice. No document, or no `glossary:` key,
// defines nothing.
func LoadGlossary(path string) ([]catalogue.Definition, error) {
	doc, found, err := readDocument(path)
	if err != nil || !found || docs.Absent(&doc.Glossary) {
		return nil, err
	}

	if doc.Glossary.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("product document %s: %w", path, ErrGlossaryNotList)
	}

	definitions := make([]catalogue.Definition, 0, len(doc.Glossary.Content))

	err = doc.Glossary.Decode(&definitions)
	if err != nil {
		return nil, fmt.Errorf("product document %s: glossary: %w", path, err)
	}

	return definitions, nil
}
