package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// stepPhrases passes a Scenario every step of which a model does not run
// matches exactly one catalogue template, declared for the step's block.
func stepPhrases() Check[*template.ScenarioCheckData] {
	return closureCheck([]string{docs.KeyProduct},
		func(item *template.ScenarioCheckData, _ Input, source product.Catalogue) (Verdict, error) {
			return ReviewPhrases(item.ID, item.Phrases, source), nil
		})
}

// ReviewPhrases holds one Item's steps to the catalogue: a malformed
// template fails every checked Item, since no step can be held to it.
// `build tests` holds every Registry Scenario to it the same way.
func ReviewPhrases(itemID string, steps []catalogue.Step, source product.Catalogue) Verdict {
	compiled, malformed := catalogue.Compile(source.Entries, source.Roles)
	if len(malformed) > 0 {
		lines := make([]string, 0, len(malformed))
		for _, malformation := range malformed {
			lines = append(lines, malformation.Line())
		}

		return Verdict{Pass: false, Context: prefixed(itemID, lines)}
	}

	review := compiled.Review(steps)
	if len(review.Findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(itemID, review.Findings)}
	}

	return Verdict{Pass: true, Context: []string{itemID + ": " + passLine(review)}}
}

// passLine says how many steps matched, and how many a model runs.
func passLine(review catalogue.Review) string {
	var line strings.Builder

	if review.Matched == 1 {
		line.WriteString("1 step matches one template")
	} else {
		fmt.Fprintf(&line, "%d steps match one template each", review.Matched)
	}

	switch {
	case review.Exempt == 1:
		line.WriteString(", 1 model-run step exempt")
	case review.Exempt > 1:
		fmt.Fprintf(&line, ", %d model-run steps exempt", review.Exempt)
	}

	return line.String()
}
