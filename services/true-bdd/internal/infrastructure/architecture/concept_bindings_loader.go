package architecture

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// LoadConceptBindings preserves duplicates for the document check to report.
func LoadConceptBindings(path string) ([]ConceptBinding, error) {
	var document struct {
		Architecture struct {
			Concepts []ConceptBinding `yaml:"concepts"`
		} `yaml:"architecture"`
	}

	_, err := docs.ReadYAML(path, "architecture document", &document)
	if err != nil {
		return nil, fmt.Errorf("read Concept bindings: %w", err)
	}

	return document.Architecture.Concepts, nil
}
