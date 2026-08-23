package inventory

import storymodel "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"

// Content is the normalized, render-ready view of a story the review modal
// shows: identity + statement + flattened Gherkin acceptance criteria (plan
// §1) — the JSON shape view-model/inventory.ts and the Go goldens pin.
type Content struct {
	ID                 string                       `json:"id"`
	Title              string                       `json:"title"`
	Status             string                       `json:"status"`
	Statement          ContentStatement             `json:"statement"`
	AcceptanceCriteria []ContentAcceptanceCriterion `json:"acceptance_criteria"`
}

// ContentStatement is the as-a / i-want / so-that user-story statement.
type ContentStatement struct {
	AsA    string `json:"as_a"`
	IWant  string `json:"i_want"`
	SoThat string `json:"so_that"`
}

// ContentAcceptanceCriterion is one AC with its flattened steps.
type ContentAcceptanceCriterion struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Steps       []ContentStep `json:"steps"`
}

// ContentStep is one flattened Gherkin step: its kind
// (given|when|then|and|but — the real model supports BOTH and AND but) and
// its exact text, in source order.
type ContentStep struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// contentFromStory flattens a typed story (a file's story: block or an
// epic's declared story) into render-ready Content, keeping each AC's steps
// in source order (and/but modifiers keep their own kind).
func contentFromStory(source storymodel.Story) *Content {
	criteria := make([]ContentAcceptanceCriterion, 0, len(source.AcceptanceCriteria))
	for _, ac := range source.AcceptanceCriteria {
		criteria = append(criteria, ContentAcceptanceCriterion{
			ID:          ac.ID,
			Description: ac.Description,
			Steps:       flattenSteps(ac.Steps),
		})
	}

	return &Content{
		ID:     source.ID,
		Title:  source.Title,
		Status: source.Status,
		Statement: ContentStatement{
			AsA:    source.AsA,
			IWant:  source.IWant,
			SoThat: source.SoThat,
		},
		AcceptanceCriteria: criteria,
	}
}

// declaredContent returns the epic-declared story's Content, or nil when
// the declaration carries no reviewable content (a bare `- id:` row) — such
// a row falls to the modal's identity-only fallback (plan §1b).
func declaredContent(source storymodel.Story) *Content {
	empty := source.Title == "" && source.AsA == "" && source.IWant == "" &&
		source.SoThat == "" && len(source.AcceptanceCriteria) == 0
	if empty {
		return nil
	}

	return contentFromStory(source)
}

// flattenSteps flattens an AC's scenario steps into ordered kind/text pairs.
func flattenSteps(steps []storymodel.ScenarioStep) []ContentStep {
	out := make([]ContentStep, 0, len(steps))
	for _, step := range steps {
		appendStatements(&out, "given", step.Given)
		appendStatements(&out, "when", step.When)
		appendStatements(&out, "then", step.Then)
	}

	return out
}

// appendStatements flattens one given/when/then array: a plain statement
// takes the block kind; an and/but modifier takes its own kind.
func appendStatements(out *[]ContentStep, kind string, statements []storymodel.StepStatement) {
	for _, statement := range statements {
		stepKind := kind
		if statement.Type != "" {
			stepKind = string(statement.Type)
		}

		*out = append(*out, ContentStep{Kind: stepKind, Text: statement.Statement})
	}
}
