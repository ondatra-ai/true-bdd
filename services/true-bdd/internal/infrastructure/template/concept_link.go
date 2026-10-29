package template

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

// ConceptLink is evidence from this walk's link turn, never a Registry annotation.
type ConceptLink struct {
	Concept string
	Block   catalogue.Block
	Intent  bool
}
