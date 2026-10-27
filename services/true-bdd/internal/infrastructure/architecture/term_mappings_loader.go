package architecture

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// ErrTermsNotList signals an `architecture.terms:` key holding something
// other than a list, in which no term could be looked up.
var ErrTermsNotList = errors.New("architecture.terms: must hold a list of mappings")

// termsDocument is the slice of the architecture document the
// term-mapping check decodes.
type termsDocument struct {
	Architecture struct {
		Terms yaml.Node `yaml:"terms"`
	} `yaml:"architecture"`
}

// LoadTermMappings reads every entry of the word mapping in the
// architecture document at path, a term mapped twice listed twice. No
// document, or no `architecture.terms:` key, maps nothing.
func LoadTermMappings(path string) ([]catalogue.Mapping, error) {
	var doc termsDocument

	found, err := docs.ReadYAML(path, "architecture document", &doc)
	if err != nil || !found {
		return nil, err
	}

	terms := &doc.Architecture.Terms
	if docs.Absent(terms) {
		return nil, nil
	}

	if terms.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("architecture document %s: %w", path, ErrTermsNotList)
	}

	mappings := make([]catalogue.Mapping, 0, len(terms.Content))

	err = terms.Decode(&mappings)
	if err != nil {
		return nil, fmt.Errorf("architecture document %s: architecture.terms: %w", path, err)
	}

	return mappings, nil
}
