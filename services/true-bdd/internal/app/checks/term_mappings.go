package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// termMappings passes a Scenario every term of which the architecture's
// word mapping maps exactly once: the terms the templates its steps match
// declare. Whether the glossary defines them is the declared-term check's.
func termMappings() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct, docs.KeyArchitectureYAML},
		func(item *template.ScenarioCheckData, input Input, source product.Catalogue) (Verdict, error) {
			mappings, err := architecture.LoadTermMappings(input.Docs[docs.KeyArchitectureYAML])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the word mapping: %w", err)
			}

			return reviewTerms(item.ID, item.Phrases, source, "mapped",
				func(declarations []catalogue.Declaration) catalogue.TermReview {
					return catalogue.ReviewMappings(declarations, mappings)
				}), nil
		})
}
