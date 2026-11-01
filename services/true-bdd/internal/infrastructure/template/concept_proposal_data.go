package template

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

// ConceptProposalData carries one concept and suite Item through its
// pattern proposal turn, and back with the engine's findings.
type ConceptProposalData struct {
	ID       string // block:concept, the Item the suite's grouping names
	Suite    string
	Service  string
	Block    catalogue.Block
	Concept  string
	Steps    []string                    // every step of the suite grouped into the concept
	Existing []string                    // patterns of the concept's existing definitions
	Suites   []catalogue.SuiteStep       // every deterministic step of the suite, every block
	Previous []catalogue.ProposedPattern // the refused proposal the findings name
	Findings []string
	Critique []string                    // the critique's findings against the patterns being replaced
	Patterns []catalogue.ProposedPattern // the proposal the engine accepted
}
