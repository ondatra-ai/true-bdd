package template

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/story"

// StoryRefineData is the Story `us refine` walks: the Story itself, inlined
// so the prompt templates read and render its fields as they always did,
// and under --fix the documents a closure fix may write.
type StoryRefineData struct {
	story.Story `yaml:",inline"`

	// TermDocuments are the documents a closure fix may write; empty
	// without --fix. A concept fix writes there, never in the Story.
	TermDocuments     []FixDocument `json:"-" yaml:"-"`
	ConceptLinks      []ConceptLink `json:"-" yaml:"-"`
	ConceptLinkPassed bool          `json:"-" yaml:"-"`
}
