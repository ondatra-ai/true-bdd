package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// reviewTerms holds the terms every declared concept relies on to one term
// document, through review; held names what a pass found each term to be
// ("defined", "mapped"). Each finding names its concept once.
func reviewTerms(itemID string, concepts []product.Concept, held string,
	review func([]catalogue.Declaration) catalogue.TermReview,
) Verdict {
	findings := make([]string, 0, len(concepts))

	for _, entry := range concepts {
		subject := fmt.Sprintf("%s concept '%s'", entry.Block, entry.Name)

		declarations := make([]catalogue.Declaration, 0, len(entry.Terms))
		for _, term := range entry.Terms {
			declarations = append(declarations, catalogue.Declaration{Concept: subject, Term: term})
		}

		findings = append(findings, review(declarations).Findings...)
	}

	if len(findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(itemID, findings)}
	}

	return Verdict{Pass: true, Context: []string{itemID + ": every concept's declared terms are " + held}}
}
