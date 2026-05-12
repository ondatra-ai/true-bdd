package template

import (
	"fmt"
	"strings"
)

// MergedSteps represents the Given-When-Then structure flattened from a
// scenario's steps[].
type MergedSteps struct {
	Given []string
	When  []string
	Then  []string
}

// ScenarioApplyData carries one acceptance criterion from a refined story
// through the apply-checklist evaluator, fix-prompt generator, and
// fix-applier templates. Each instance represents a single (scenario,
// prompt) cell input for the apply walk.
type ScenarioApplyData struct {
	StoryID                 string      // e.g. "4.1"
	StoryPath               string      // e.g. "docs/stories/4.1-shared-document-editing.yaml"
	ACID                    string      // e.g. "AC-1"
	LineageScenarioID       string      // e.g. "4.1-001" — matches user_stories[].scenario_id
	Description             string      // verbatim AC description
	Steps                   MergedSteps // Given / When / Then flattened from AC steps
	RequirementsScratchPath string      // tmp copy of docs/requirements.yaml the run mutates
}

// NewScenarioApplyData builds a ScenarioApplyData for one AC.
func NewScenarioApplyData(
	storyID string,
	storyPath string,
	acID string,
	lineageScenarioID string,
	description string,
	given []string,
	when []string,
	then []string,
	requirementsScratchPath string,
) *ScenarioApplyData {
	return &ScenarioApplyData{
		StoryID:           storyID,
		StoryPath:         storyPath,
		ACID:              acID,
		LineageScenarioID: lineageScenarioID,
		Description:       description,
		Steps: MergedSteps{
			Given: given,
			When:  when,
			Then:  then,
		},
		RequirementsScratchPath: requirementsScratchPath,
	}
}

// FormatSteps renders the AC's Given / When / Then for display in the
// apply prompt templates.
func (d *ScenarioApplyData) FormatSteps() string {
	var result strings.Builder

	if len(d.Steps.Given) > 0 {
		result.WriteString("Given:\n")

		for _, step := range d.Steps.Given {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	if len(d.Steps.When) > 0 {
		result.WriteString("When:\n")

		for _, step := range d.Steps.When {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	if len(d.Steps.Then) > 0 {
		result.WriteString("Then:\n")

		for _, step := range d.Steps.Then {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	return result.String()
}
