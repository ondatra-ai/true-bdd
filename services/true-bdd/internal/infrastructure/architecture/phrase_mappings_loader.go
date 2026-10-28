package architecture

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// LoadPhraseMappings preserves duplicate entries so the check can name them.
func LoadPhraseMappings(path string) ([]PhraseMapping, error) {
	var document struct {
		Architecture struct {
			Phrases []PhraseMapping `yaml:"phrases"`
		} `yaml:"architecture"`
	}

	_, err := docs.ReadYAML(path, "architecture document", &document)
	if err != nil {
		return nil, fmt.Errorf("read Phrase mapping: %w", err)
	}

	return document.Architecture.Phrases, nil
}
