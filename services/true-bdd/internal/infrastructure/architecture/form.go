package architecture

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// ErrPhrasesRetired refuses an architecture document still declaring the
// retired Phrase mapping, naming what replaced it.
var ErrPhrasesRetired = errors.New("architecture.phrases is no longer read; " +
	"bind each concept in architecture.concepts in its place (ADR-0035)")

// ValidateForm refuses the retired Phrase mapping before a command starts. An
// absent or unreadable document is left to the command that reads it.
func ValidateForm(path string) error {
	var document struct {
		Architecture struct {
			Phrases yaml.Node `yaml:"phrases"`
		} `yaml:"architecture"`
	}

	found, err := docs.ReadYAML(path, "architecture document", &document)
	if err != nil || !found {
		return nil //nolint:nilerr // The command reports an unreadable architecture as it always did.
	}

	if document.Architecture.Phrases.Kind != 0 {
		return fmt.Errorf("architecture document %s: %w", path, ErrPhrasesRetired)
	}

	return nil
}
