package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// Why a Scenario passes every closure check unchecked.
const (
	notCheckedNoCatalogue = "the product document declares no catalogue"
	notCheckedUnclaimed   = "no Story claims this Scenario yet"
)

// closureReview is what one closure check holds a checked Scenario to,
// given the catalogue its product document declares.
type closureReview func(item *template.ScenarioCheckData, input Input, source product.Catalogue) (Verdict, error)

// closureCheck is a closure check reading the documents of docKeys: it
// reads the catalogue afresh at every Cell, so a fix shows at once, passes
// an Item the activation contract leaves unchecked, and reviews the rest.
func closureCheck(docKeys []string, review closureReview) Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Params: nil,
		Docs:   docKeys,
		Run: func(item *template.ScenarioCheckData, input Input) (Verdict, error) {
			source, err := product.LoadCatalogue(input.Docs[docs.KeyProduct])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the catalogue: %w", err)
			}

			item.ConceptForm = source.ConceptsDeclared

			verdict, unchecked := activation(item, source)
			if unchecked {
				return verdict, nil
			}

			return review(item, input, source)
		},
	}
}

// activation is the verdict of an Item every closure check leaves
// unchecked, and true, or false when the check must read the Item.
func activation(item *template.ScenarioCheckData, source product.Catalogue) (Verdict, bool) {
	switch {
	case source.ConceptsDeclared:
		return Verdict{}, false
	case !source.Declared:
		return notChecked(item.ID, notCheckedNoCatalogue), true
	case !claimed(item):
		return notChecked(item.ID, notCheckedUnclaimed), true
	}

	return Verdict{}, false
}

// claimed reports whether a Story claims the Scenario, which is what puts
// it under the closure checks once a catalogue is declared.
func claimed(item *template.ScenarioCheckData) bool {
	return len(item.UserStories) > 0
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
