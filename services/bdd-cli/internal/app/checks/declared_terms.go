package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// declaredTerms passes a Scenario every term of which the product glossary
// defines exactly once: the terms the templates its steps match declare.
// A step matching no template, or several, is the step-phrase check's finding.
func declaredTerms() Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Params: nil,
		Docs:   []string{docs.KeyProduct},
		Run: func(item *template.ScenarioCheckData, input Input) (Verdict, error) {
			path := input.Docs[docs.KeyProduct]

			source, err := product.LoadCatalogue(path)
			if err != nil {
				return Verdict{}, fmt.Errorf("read the catalogue: %w", err)
			}

			verdict, unchecked := activation(item, source)
			if unchecked {
				return verdict, nil
			}

			definitions, err := product.LoadGlossary(path)
			if err != nil {
				return Verdict{}, fmt.Errorf("read the glossary: %w", err)
			}

			return reviewTerms(item.ID, item.Phrases, source, definitions), nil
		},
	}
}

// reviewTerms holds the terms one Item's steps rely on to the glossary.
// A template the grammar refuses declares nothing here.
func reviewTerms(
	subjectID string,
	steps []catalogue.Step,
	source product.Catalogue,
	definitions []catalogue.Definition,
) Verdict {
	compiled, _ := catalogue.Compile(source.Entries, source.Roles)

	review := catalogue.ReviewTerms(compiled.Declarations(steps), definitions)
	if len(review.Findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(subjectID, review.Findings)}
	}

	if len(review.Terms) == 0 {
		return Verdict{Pass: true, Context: []string{subjectID + ": no matched template declares a term"}}
	}

	return Verdict{Pass: true, Context: []string{
		subjectID + ": every declared term is defined: " + strings.Join(review.Terms, ", "),
	}}
}
