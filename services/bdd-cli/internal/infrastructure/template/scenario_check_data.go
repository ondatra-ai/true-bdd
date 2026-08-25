package template

import (
	"fmt"
	"strings"
)

// StoryRef is one entry of a scenario's `user_stories[]` lineage, in the
// shape the scen-check prompts read it.
type StoryRef struct {
	Story      string
	ScenarioID string
}

// ScenarioCheckData carries one registry scenario through the scen-check
// evaluator: one instance per (scenario, prompt) cell input. It holds the
// scenario's own fields and no path to the registry — see docs/adr/0001.
type ScenarioCheckData struct {
	ID          string      // e.g. "E2E-015"
	Description string      // verbatim registry description
	Service     string      // names an entry in architecture services[]
	Path        string      // generated test file the scenario renders into
	UserStories []StoryRef  // lineage back to the stories it was merged from
	Steps       MergedSteps // Given / When / Then as the registry holds them
}

// FormatSteps renders the scenario's Given / When / Then for the
// scen-check prompt templates.
func (d *ScenarioCheckData) FormatSteps() string {
	return formatMergedSteps(d.Steps)
}

// FormatUserStories renders the lineage list. An empty list gets a line
// of its own: "no story claims it yet" is a fact about the scenario, and
// a blank would read as a rendering failure.
func (d *ScenarioCheckData) FormatUserStories() string {
	if len(d.UserStories) == 0 {
		return "(none — no story claims this scenario)\n"
	}

	var result strings.Builder

	for _, ref := range d.UserStories {
		fmt.Fprintf(&result, "  - %s (%s)\n", ref.Story, ref.ScenarioID)
	}

	return result.String()
}
