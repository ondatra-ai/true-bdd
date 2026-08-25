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
// fix-applier templates: one instance per (scenario, prompt) cell input.
type ScenarioApplyData struct {
	StoryID                 string      // e.g. "4.1"
	StoryPath               string      // e.g. "docs/product/stories/4.1-shared-document-editing.yaml"
	ACID                    string      // e.g. "AC-1"
	LineageScenarioID       string      // e.g. "4.1-001" — matches user_stories[].scenario_id
	Description             string      // verbatim AC description
	Steps                   MergedSteps // Given / When / Then flattened from AC steps
	RequirementsScratchPath string      // tmp copy of docs/scenarios.yaml the run mutates
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
	return formatMergedSteps(d.Steps)
}

// formatMergedSteps renders a Given / When / Then block, omitting any
// kind the subject has none of. Shared by every subject type whose
// templates display steps.
func formatMergedSteps(steps MergedSteps) string {
	var result strings.Builder

	for _, block := range []struct {
		label string
		lines []string
	}{
		{"Given", steps.Given},
		{"When", steps.When},
		{"Then", steps.Then},
	} {
		if len(block.lines) == 0 {
			continue
		}

		result.WriteString(block.label + ":\n")

		for _, step := range block.lines {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	return result.String()
}
