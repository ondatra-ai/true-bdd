package models

import "bdd-cli/src/internal/domain/models/story"

type EpicDocument struct {
	Epic            EpicInfo      `yaml:"epic"`
	Stories         []story.Story `yaml:"stories"`
	Dependencies    []string      `yaml:"dependencies"`
	SuccessCriteria []string      `yaml:"success_criteria"`
	TechnicalNotes  []string      `yaml:"technical_notes"`
}

type EpicInfo struct {
	ID                int    `yaml:"id"`
	Name              string `yaml:"name"`
	Status            string `yaml:"status"`
	Goal              string `yaml:"goal"`
	CompletionSummary string `yaml:"completion_summary,omitempty"`
	Context           string `yaml:"context"`
}
