package story_test

import (
	"testing"

	"bdd-cli/src/internal/domain/models/story"

	"gopkg.in/yaml.v3"
)

func TestStepStatementUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected story.StepStatement
	}{
		{
			name:     "plain string",
			yaml:     `"Server is ready"`,
			expected: story.StepStatement{Type: "", Statement: "Server is ready"},
		},
		{
			name:     "and modifier",
			yaml:     `and: "Authentication configured"`,
			expected: story.StepStatement{Type: story.ModifierTypeAnd, Statement: "Authentication configured"},
		},
		{
			name:     "but modifier",
			yaml:     `but: "No requests made"`,
			expected: story.StepStatement{Type: story.ModifierTypeBut, Statement: "No requests made"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var stmt story.StepStatement

			err := yaml.Unmarshal([]byte(testCase.yaml), &stmt)
			if err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if stmt.Type != testCase.expected.Type {
				t.Errorf("Type = %v, want %v", stmt.Type, testCase.expected.Type)
			}

			if stmt.Statement != testCase.expected.Statement {
				t.Errorf("Statement = %v, want %v", stmt.Statement, testCase.expected.Statement)
			}
		})
	}
}

func TestScenarioStepUnmarshal(t *testing.T) {
	yamlData := `
given:
  - "Server is ready to accept connections"
  - and: "Authentication is configured"
when:
  - "Client attempts to connect"
then:
  - "Server accepts connection"
  - and: "Welcome message sent"
`

	var step story.ScenarioStep

	err := yaml.Unmarshal([]byte(yamlData), &step)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Check Given
	if len(step.Given) != 2 {
		t.Errorf("Given length = %d, want 2", len(step.Given))
	}

	if step.Given[0].Type != "" {
		t.Errorf("Given[0].Type = %v, want empty", step.Given[0].Type)
	}

	if step.Given[0].Statement != "Server is ready to accept connections" {
		t.Errorf("Given[0].Statement = %v", step.Given[0].Statement)
	}

	if step.Given[1].Type != story.ModifierTypeAnd {
		t.Errorf("Given[1].Type = %v, want and", step.Given[1].Type)
	}

	// Check When
	if len(step.When) != 1 {
		t.Errorf("When length = %d, want 1", len(step.When))
	}

	// Check Then
	if len(step.Then) != 2 {
		t.Errorf("Then length = %d, want 2", len(step.Then))
	}
}
