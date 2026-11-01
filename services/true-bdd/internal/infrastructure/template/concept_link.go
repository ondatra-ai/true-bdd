package template

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

// ConceptLink is evidence from this walk's link turn, never a Registry annotation.
type ConceptLink struct {
	Concept string
	Block   catalogue.Block
	Intent  bool
}

// Key names the linked concept within its block, as its binding and words are judged.
func (l ConceptLink) Key() string {
	return catalogue.ConceptKey(l.Block, l.Concept)
}
