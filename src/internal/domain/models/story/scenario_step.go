package story

import (
	"fmt"

	"gopkg.in/yaml.v3"

	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

// ModifierType represents the type of statement modifier.
type ModifierType string

const (
	ModifierTypeAnd ModifierType = "and"
	ModifierTypeBut ModifierType = "but"

	yamlMappingNodeContentSize = 2 // YAML mapping nodes have 2 elements: key + value
)

// StepStatement represents a single statement in a Gherkin step
// Can be either a main statement (plain string) or a modifier (and/but).
type StepStatement struct {
	Type      ModifierType // Empty for main statement, "and" or "but" for modifiers
	Statement string
}

// UnmarshalYAML implements custom YAML unmarshaling
// Handles both plain strings and objects with and/but keys.
func (s *StepStatement) UnmarshalYAML(node *yaml.Node) error {
	// Handle plain string (main statement)
	if node.Kind == yaml.ScalarNode {
		s.Type = ""
		s.Statement = node.Value

		return nil
	}

	// Handle object with and/but key
	if node.Kind == yaml.MappingNode {
		if len(node.Content) != yamlMappingNodeContentSize {
			return fmt.Errorf("modifier validation error: %w", pkgerrors.ErrModifierMustHaveOneKey)
		}

		key := node.Content[0].Value
		value := node.Content[1].Value

		switch key {
		case "and":
			s.Type = ModifierTypeAnd
			s.Statement = value
		case "but":
			s.Type = ModifierTypeBut
			s.Statement = value
		default:
			return fmt.Errorf("invalid modifier: %w", pkgerrors.ErrInvalidModifierError(key))
		}

		return nil
	}

	return fmt.Errorf("invalid step statement format: %w", pkgerrors.ErrInvalidStepStatementFormat)
}

// MarshalYAML implements custom YAML marshaling
// Outputs plain string for main statement, object for modifiers.
func (s StepStatement) MarshalYAML() (interface{}, error) {
	// Main statement: output as plain string
	if s.Type == "" {
		return s.Statement, nil
	}

	// Modifier: output as object with and/but key
	return map[string]string{
		string(s.Type): s.Statement,
	}, nil
}

// ScenarioStep represents a single step in a Gherkin scenario
// Each Given/When/Then is an array of statements.
type ScenarioStep struct {
	Given []StepStatement `json:"given,omitempty" yaml:"given,omitempty"`
	When  []StepStatement `json:"when,omitempty"  yaml:"when,omitempty"`
	Then  []StepStatement `json:"then,omitempty"  yaml:"then,omitempty"`
}
