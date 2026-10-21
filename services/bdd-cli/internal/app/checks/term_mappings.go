package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// termMappings passes a Scenario every term of which the architecture's
// word mapping maps exactly once: the terms the templates its steps match
// declare. Whether the glossary defines them is the declared-term check's.
func termMappings() Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Params: nil,
		Docs:   []string{docs.KeyProduct, docs.KeyArchitectureYAML},
		Run: func(item *template.ScenarioCheckData, input Input) (Verdict, error) {
			source, err := product.LoadCatalogue(input.Docs[docs.KeyProduct])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the catalogue: %w", err)
			}

			verdict, unchecked := activation(item, source)
			if unchecked {
				return verdict, nil
			}

			mappings, err := architecture.LoadTermMappings(input.Docs[docs.KeyArchitectureYAML])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the word mapping: %w", err)
			}

			return reviewMappings(item.ID, item.Phrases, source, mappings), nil
		},
	}
}

// reviewMappings holds the terms one Item's steps rely on to the word
// mapping. A template the grammar refuses declares nothing here.
func reviewMappings(
	subjectID string,
	steps []catalogue.Step,
	source product.Catalogue,
	mappings []catalogue.Mapping,
) Verdict {
	compiled, _ := catalogue.Compile(source.Entries, source.Roles)

	review := catalogue.ReviewMappings(compiled.Declarations(steps), mappings)
	if len(review.Findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(subjectID, review.Findings)}
	}

	if len(review.Terms) == 0 {
		return Verdict{Pass: true, Context: []string{subjectID + ": no matched template declares a term"}}
	}

	return Verdict{Pass: true, Context: []string{
		subjectID + ": every declared term is mapped: " + strings.Join(review.Terms, ", "),
	}}
}
