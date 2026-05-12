package story

import (
	"log/slog"

	"bdd-cli/src/internal/infrastructure/docs"
)

type StoryDocument struct {
	Story            Story                  `json:"story"                yaml:"story"`
	Scenarios        Scenarios              `json:"scenarios"            yaml:"scenarios"`
	ChangeLog        []ChangeLogEntry       `json:"change_log"           yaml:"change_log"`
	QAResults        *QAResults             `json:"qa_results,omitempty" yaml:"qa_results,omitempty"`
	DevAgentRecord   DevAgentRecord         `json:"dev_agent_record"     yaml:"dev_agent_record"`
	ArchitectureDocs *docs.ArchitectureDocs `json:"-"                    yaml:"-"`
}

// EnsureScenariosPopulated generates TestScenarios from acceptance criteria
// when the scenarios section is empty. This bridges the newer AC-with-steps
// format to the existing merge_scenarios pipeline.
func (d *StoryDocument) EnsureScenariosPopulated() {
	if len(d.Scenarios.TestScenarios) > 0 {
		return
	}

	var acsWithSteps []AcceptanceCriterion

	for _, ac := range d.Story.AcceptanceCriteria {
		if len(ac.Steps) > 0 {
			acsWithSteps = append(acsWithSteps, ac)
		}
	}

	if len(acsWithSteps) == 0 {
		return
	}

	parser := &GherkinParser{}

	scenarios, err := parser.GenerateScenarios(d.Story.ID, acsWithSteps)
	if err != nil {
		slog.Warn("Failed to generate scenarios from acceptance criteria", "error", err)

		return
	}

	d.Scenarios.TestScenarios = scenarios

	slog.Debug("Generated scenarios from acceptance criteria",
		"count", len(scenarios),
		"story_id", d.Story.ID,
	)
}
