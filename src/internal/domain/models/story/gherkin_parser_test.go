package story_test

import (
	"fmt"
	"strings"
	"testing"

	"bdd-cli/src/internal/domain/models/story"

	"gopkg.in/yaml.v3"
)

const (
	ac1 = "AC-1"
	ac2 = "AC-2"
)

func TestGenerateScenarios_MultipleACsWithSteps(t *testing.T) {
	parser := &story.GherkinParser{}

	givenShared := "Claude User has shared a Google Doc as Editor"
	thenReflects := "document content reflects the requested changes"

	acs := []story.AcceptanceCriterion{
		{
			ID:          ac1,
			Description: "Claude must modify a shared Google Doc",
			Steps: []story.ScenarioStep{
				{Given: []story.StepStatement{{Statement: givenShared}}},
				{When: []story.StepStatement{{Statement: "Claude User asks Claude to edit"}}},
				{Then: []story.StepStatement{
					{Statement: "MCP tool returns a success response"},
					{Type: story.ModifierTypeAnd, Statement: thenReflects},
				}},
			},
		},
		{
			ID:          ac2,
			Description: "Claude should respond with sharing instructions",
			Steps: []story.ScenarioStep{
				{Given: []story.StepStatement{{Statement: "a Google Doc not shared"}}},
				{When: []story.StepStatement{{Statement: "Claude User asks Claude to edit"}}},
				{Then: []story.StepStatement{
					{Statement: "Claude responds with the service account email"},
					{Type: story.ModifierTypeAnd, Statement: "includes instructions to add as Editor"},
				}},
			},
		},
	}

	scenarios, err := parser.GenerateScenarios("4.1", acs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}

	assertScenarioMeta(t, scenarios[0], "4.1-001", ac1)
	assertScenarioMeta(t, scenarios[1], "4.1-002", ac2)

	// Verify steps are copied from AC
	if len(scenarios[0].Steps) != 3 {
		t.Fatalf("expected 3 steps in scenario 0, got %d", len(scenarios[0].Steps))
	}

	assertStatement(t, scenarios[0].Steps[0].Given, 0, "", givenShared)
	assertStatement(t, scenarios[0].Steps[2].Then, 1,
		string(story.ModifierTypeAnd), thenReflects)
}

func TestGenerateScenarios_ErrorOnMissingSteps(t *testing.T) {
	parser := &story.GherkinParser{}

	acs := []story.AcceptanceCriterion{
		{
			ID:          ac1,
			Description: "Claude must do something",
			Steps: []story.ScenarioStep{
				{Given: []story.StepStatement{{Statement: "a precondition"}}},
				{When: []story.StepStatement{{Statement: "an action"}}},
				{Then: []story.StepStatement{{Statement: "an outcome"}}},
			},
		},
		{
			ID:          ac2,
			Description: "Claude should do something else",
			// No steps — should cause error
		},
	}

	scenarios, err := parser.GenerateScenarios("5.1", acs)
	if err == nil {
		t.Fatal("expected error for AC without steps, got nil")
	}

	if scenarios != nil {
		t.Errorf("expected nil scenarios on error, got %v", scenarios)
	}

	if !strings.Contains(err.Error(), ac2) {
		t.Errorf("error should reference AC-2, got: %v", err)
	}
}

func TestGenerateScenarios_EmptyFieldsLeftBlank(t *testing.T) {
	parser := &story.GherkinParser{}

	acs := []story.AcceptanceCriterion{
		{
			ID:          ac1,
			Description: "Claude must do something",
			Steps: []story.ScenarioStep{
				{Given: []story.StepStatement{{Statement: "a user"}}},
				{When: []story.StepStatement{{Statement: "they act"}}},
				{Then: []story.StepStatement{{Statement: "result"}}},
			},
		},
	}

	scenarios, err := parser.GenerateScenarios("1.0", acs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if scenarios[0].Level != "" || scenarios[0].Priority != "" || scenarios[0].Service != "" {
		t.Errorf("expected empty level/priority/service, got %q/%q/%q",
			scenarios[0].Level, scenarios[0].Priority, scenarios[0].Service)
	}
}

func TestGenerateScenarios_SequentialIDs(t *testing.T) {
	parser := &story.GherkinParser{}

	acs := make([]story.AcceptanceCriterion, 3)
	for i := range acs {
		acs[i] = story.AcceptanceCriterion{
			ID:          fmt.Sprintf("AC-%d", i+1),
			Description: "some requirement",
			Steps: []story.ScenarioStep{
				{Given: []story.StepStatement{{Statement: "precondition"}}},
				{When: []story.StepStatement{{Statement: "action"}}},
				{Then: []story.StepStatement{{Statement: "outcome"}}},
			},
		}
	}

	scenarios, err := parser.GenerateScenarios("3.2", acs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedIDs := []string{"3.2-001", "3.2-002", "3.2-003"}
	for i, s := range scenarios {
		if s.ID != expectedIDs[i] {
			t.Errorf("scenario[%d].ID = %q, want %q", i, s.ID, expectedIDs[i])
		}
	}
}

func TestGenerateScenarios_YAMLRoundTrip(t *testing.T) {
	parser := &story.GherkinParser{}

	acs := []story.AcceptanceCriterion{
		{
			ID:          ac1,
			Description: "Claude must be able to modify a Google Doc shared with the service account",
			Steps: []story.ScenarioStep{
				{Given: []story.StepStatement{
					{Statement: "Claude User has a Google Doc"},
					{Type: story.ModifierTypeAnd, Statement: "Claude User has shared the Google Doc with the service account"},
				}},
				{When: []story.StepStatement{
					{Statement: "Claude User asks Claude to modify the document content"},
				}},
				{Then: []story.StepStatement{
					{Statement: "the Google Doc content reflects the modification"},
				}},
			},
		},
	}

	scenarios, err := parser.GenerateScenarios("4.1", acs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}

	// Marshal to YAML and unmarshal back
	data, err := yaml.Marshal(scenarios[0])
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var roundTripped story.TestScenario

	err = yaml.Unmarshal(data, &roundTripped)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	verifyRoundTrippedScenario(t, roundTripped, string(data))
}

func TestAcceptanceCriterion_YAMLRoundTrip(t *testing.T) {
	criterion := story.AcceptanceCriterion{
		ID:          ac1,
		Description: "Claude must modify a shared document",
		Steps: []story.ScenarioStep{
			{Given: []story.StepStatement{{Statement: "a shared document"}}},
			{When: []story.StepStatement{{Statement: "Claude edits it"}}},
			{Then: []story.StepStatement{
				{Statement: "changes are applied"},
				{Type: story.ModifierTypeAnd, Statement: "document is updated"},
			}},
		},
	}

	data, err := yaml.Marshal(criterion)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var roundTripped story.AcceptanceCriterion

	err = yaml.Unmarshal(data, &roundTripped)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if roundTripped.ID != ac1 {
		t.Errorf("ID = %q, want %q", roundTripped.ID, ac1)
	}

	if roundTripped.Description != "Claude must modify a shared document" {
		t.Errorf("Description = %q, want %q", roundTripped.Description, "Claude must modify a shared document")
	}

	if len(roundTripped.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(roundTripped.Steps))
	}

	// Verify YAML contains both description and steps
	yamlStr := string(data)
	for _, key := range []string{"description:", "steps:", "given:", "when:", "then:", "and:"} {
		if !strings.Contains(yamlStr, key) {
			t.Errorf("YAML output missing %q key", key)
		}
	}
}

// verifyRoundTrippedScenario checks the structure after YAML round-trip.
func verifyRoundTrippedScenario(t *testing.T, scenario story.TestScenario, yamlOutput string) {
	t.Helper()

	if scenario.ID != "4.1-001" {
		t.Errorf("ID = %q, want %q", scenario.ID, "4.1-001")
	}

	if len(scenario.Steps) != 3 {
		t.Fatalf("expected 3 steps after round-trip, got %d", len(scenario.Steps))
	}

	// Given step: main statement + And modifier
	if len(scenario.Steps[0].Given) != 2 {
		t.Fatalf("expected 2 Given statements, got %d", len(scenario.Steps[0].Given))
	}

	assertStatement(t, scenario.Steps[0].Given, 0, "", "Claude User has a Google Doc")
	assertStatement(t, scenario.Steps[0].Given, 1, string(story.ModifierTypeAnd),
		"Claude User has shared the Google Doc with the service account")

	// Verify YAML format contains expected keys
	for _, key := range []string{"given:", "when:", "then:", "and:"} {
		if !strings.Contains(yamlOutput, key) {
			t.Errorf("YAML output missing %q key", key)
		}
	}
}

// assertStatement verifies a single StepStatement at the given index.
func assertStatement(t *testing.T, stmts []story.StepStatement, idx int, wantType string, wantText string) {
	t.Helper()

	if idx >= len(stmts) {
		t.Fatalf("index %d out of range (len=%d)", idx, len(stmts))
	}

	if string(stmts[idx].Type) != wantType {
		t.Errorf("stmts[%d].Type = %q, want %q", idx, stmts[idx].Type, wantType)
	}

	if stmts[idx].Statement != wantText {
		t.Errorf("stmts[%d].Statement = %q, want %q", idx, stmts[idx].Statement, wantText)
	}
}

// assertScenarioMeta verifies ID and AC reference of a scenario.
func assertScenarioMeta(t *testing.T, scenario story.TestScenario, wantID string, wantAC string) {
	t.Helper()

	if scenario.ID != wantID {
		t.Errorf("scenario.ID = %q, want %q", scenario.ID, wantID)
	}

	if len(scenario.AcceptanceCriteria) != 1 || scenario.AcceptanceCriteria[0] != wantAC {
		t.Errorf("scenario.AcceptanceCriteria = %v, want [%s]", scenario.AcceptanceCriteria, wantAC)
	}
}
