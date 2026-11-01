package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// notCheckedNoConcepts is why a Scenario passes every closure check unchecked.
const notCheckedNoConcepts = "the product document declares no concepts"

// closureReview is what one closure check holds a checked Scenario to,
// given the concepts its product document declares.
type closureReview func(item *template.ScenarioCheckData, input Input, source product.Document) (Verdict, error)

// closureCheck reads the concepts afresh at every Cell, so a fix shows at
// once; it passes every Scenario of a host declaring none unchecked and
// reviews every other Scenario, a Story claiming it or not.
func closureCheck(docKeys []string, review closureReview) Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Params: nil,
		Docs:   docKeys,
		Run: func(item *template.ScenarioCheckData, input Input) (Verdict, error) {
			source, err := product.Load(input.Docs[docs.KeyProduct])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the concepts: %w", err)
			}

			item.ConceptsDeclared = source.ConceptsDeclared
			if !source.ConceptsDeclared {
				return notChecked(item.ID, notCheckedNoConcepts), nil
			}

			return review(item, input, source)
		},
	}
}

// notChecked passes an Item the activation contract leaves unchecked,
// saying why, so a pass that checked nothing never reads as one that did.
func notChecked(itemID, reason string) Verdict {
	return Verdict{Pass: true, Context: []string{itemID + ": not checked: " + reason}}
}

// prefixed heads each finding with the Item's id, as a context line reads.
func prefixed(itemID string, findings []string) []string {
	context := make([]string, 0, len(findings))
	for _, finding := range findings {
		context = append(context, itemID+": "+finding)
	}

	return context
}
