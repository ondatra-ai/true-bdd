package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

func reviewConceptTerms(itemID string, concepts []product.Concept, held string,
	review func([]catalogue.Declaration) catalogue.TermReview,
) Verdict {
	findings := make([]string, 0)

	for _, entry := range concepts {
		subject := fmt.Sprintf("%s concept '%s'", entry.Block, entry.Name)

		declarations := make([]catalogue.Declaration, 0, len(entry.Terms))
		for _, term := range entry.Terms {
			declarations = append(declarations, catalogue.Declaration{Template: subject, Term: term})
		}

		result := review(declarations)
		for _, finding := range result.Findings {
			findings = append(findings, subject+": "+finding)
		}
	}

	if len(findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(itemID, findings)}
	}

	return Verdict{Pass: true, Context: []string{itemID + ": every concept's declared terms are " + held}}
}
