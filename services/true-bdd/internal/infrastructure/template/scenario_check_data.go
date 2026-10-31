package template

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// StoryRef is one entry of a scenario's `user_stories[]` lineage, in the
// shape the scen-check prompts read it.
type StoryRef struct {
	Story      string
	ScenarioID string
}

// ScenarioCheckData carries one registry scenario through the scen-check
// evaluator: one instance per (scenario, prompt) cell input. It holds the
// scenario's own fields, the whole Registry, and under --fix its working copy.
type ScenarioCheckData struct {
	ConceptLinks      []ConceptLink // The current walk only; never stored in the Registry.
	ConceptLinkPassed bool
	ConceptsDeclared  bool        // the product document declares concepts
	ID                string      // e.g. "scen-check-003-unknown-id-refused"
	Description       string      // verbatim registry description
	Service           string      // names an entry in architecture services[]
	Path              string      // generated test file the scenario renders into
	UserStories       []StoryRef  // lineage back to the stories it was merged from
	Steps             MergedSteps // Given / When / Then as the registry holds them
	// CheckedSteps are the same steps as the closure checks read them: each
	// one's own text, without its and:/but: keyword, beside its block.
	CheckedSteps []catalogue.Step
	// TermDocuments are the documents a closure fix may write; empty
	// without --fix. A fix defines a term there, never in the Scenario.
	TermDocuments []FixDocument
	// InterfaceDocuments are the working copies of the Public interface
	// documents the architecture names, which a public-interface or
	// concept-bindings fix may extend; empty without --fix.
	InterfaceDocuments []FixDocument
	// RegistryDocument is the Registry's working copy under --fix, where
	// a model Prompt's fix rewrites the Scenario; nil without --fix.
	RegistryDocument *FixDocument
	// Registry is every Scenario of the Registry, this one included,
	// whatever the walk selected: a check relating Scenarios reads it.
	Registry []*ScenarioCheckData
}

// Writes reports whether a fix in this run may write the term document
// of that key, so a template describes only the working copies it has.
func (d *ScenarioCheckData) Writes(key string) bool {
	for _, document := range d.TermDocuments {
		if document.Key == key {
			return true
		}
	}

	return false
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
