package story_test

import (
	"testing"

	"bdd-cli/src/internal/domain/models/story"
)

func TestEnsureScenariosPopulated_PreservesExisting(t *testing.T) {
	doc := &story.StoryDocument{
		Scenarios: story.Scenarios{
			TestScenarios: []story.TestScenario{{ID: "existing-001"}},
		},
		Story: story.Story{
			ID: "1.0",
			AcceptanceCriteria: []story.AcceptanceCriterion{
				{ID: ac1, Steps: []story.ScenarioStep{{Given: []story.StepStatement{{Statement: "x"}}}}},
			},
		},
	}

	doc.EnsureScenariosPopulated()

	if len(doc.Scenarios.TestScenarios) != 1 || doc.Scenarios.TestScenarios[0].ID != "existing-001" {
		t.Errorf("expected existing scenarios to be preserved, got %v", doc.Scenarios.TestScenarios)
	}
}

func TestEnsureScenariosPopulated_GeneratesFromACsWithSteps(t *testing.T) {
	doc := &story.StoryDocument{
		Story: story.Story{
			ID: "4.1",
			AcceptanceCriteria: []story.AcceptanceCriterion{
				{
					ID: ac1,
					Steps: []story.ScenarioStep{
						{Given: []story.StepStatement{{Statement: "a document"}}},
					},
				},
				{
					ID: ac2,
					Steps: []story.ScenarioStep{
						{When: []story.StepStatement{{Statement: "user edits"}}},
					},
				},
			},
		},
	}

	doc.EnsureScenariosPopulated()

	if len(doc.Scenarios.TestScenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(doc.Scenarios.TestScenarios))
	}

	if doc.Scenarios.TestScenarios[0].ID != "4.1-001" {
		t.Errorf("expected ID 4.1-001, got %s", doc.Scenarios.TestScenarios[0].ID)
	}

	if doc.Scenarios.TestScenarios[1].ID != "4.1-002" {
		t.Errorf("expected ID 4.1-002, got %s", doc.Scenarios.TestScenarios[1].ID)
	}
}

func TestEnsureScenariosPopulated_FiltersACsWithoutSteps(t *testing.T) {
	doc := &story.StoryDocument{
		Story: story.Story{
			ID: "2.0",
			AcceptanceCriteria: []story.AcceptanceCriterion{
				{ID: ac1, Description: "no steps here"},
				{
					ID: ac2,
					Steps: []story.ScenarioStep{
						{Then: []story.StepStatement{{Statement: "result"}}},
					},
				},
				{ID: "AC-3", Description: "also no steps"},
			},
		},
	}

	doc.EnsureScenariosPopulated()

	if len(doc.Scenarios.TestScenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(doc.Scenarios.TestScenarios))
	}

	if doc.Scenarios.TestScenarios[0].AcceptanceCriteria[0] != ac2 {
		t.Errorf("expected AC-2, got %s", doc.Scenarios.TestScenarios[0].AcceptanceCriteria[0])
	}
}

func TestEnsureScenariosPopulated_NoACsWithSteps(t *testing.T) {
	doc := &story.StoryDocument{
		Story: story.Story{
			ID: "3.0",
			AcceptanceCriteria: []story.AcceptanceCriterion{
				{ID: ac1, Description: "no steps"},
				{ID: ac2, Description: "no steps either"},
			},
		},
	}

	doc.EnsureScenariosPopulated()

	if len(doc.Scenarios.TestScenarios) != 0 {
		t.Errorf("expected 0 scenarios, got %d", len(doc.Scenarios.TestScenarios))
	}
}

func TestEnsureScenariosPopulated_NoACsAtAll(t *testing.T) {
	doc := &story.StoryDocument{
		Story: story.Story{ID: "5.0"},
	}

	doc.EnsureScenariosPopulated()

	if len(doc.Scenarios.TestScenarios) != 0 {
		t.Errorf("expected 0 scenarios, got %d", len(doc.Scenarios.TestScenarios))
	}
}
