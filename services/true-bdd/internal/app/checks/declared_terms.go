package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// declaredTerms passes a Scenario every term of which the product glossary
// defines exactly once: the terms the templates its steps match declare.
// A step matching no template, or several, is the step-phrase check's finding.
func declaredTerms() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct},
		func(item *template.ScenarioCheckData, input Input, source product.Catalogue) (Verdict, error) {
			definitions, err := product.LoadGlossary(input.Docs[docs.KeyProduct])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the glossary: %w", err)
			}

			return reviewTerms(item.ID, item.Phrases, source, "defined",
				func(declarations []catalogue.Declaration) catalogue.TermReview {
					return catalogue.ReviewTerms(declarations, definitions)
				}), nil
		})
}
