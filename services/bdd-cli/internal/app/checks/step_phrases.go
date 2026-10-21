package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// Why a Scenario passes every closure check unchecked.
const (
	notCheckedNoCatalogue = "the product document declares no catalogue"
	notCheckedUnclaimed   = "no Story claims this Scenario yet"
)

// stepPhrases passes a Scenario every step of which a model does not run
// matches exactly one catalogue template, declared for the step's block.
// It reads the product document afresh at every Cell, so a fix shows at once.
func stepPhrases() Check[*template.ScenarioCheckData] {
	return Check[*template.ScenarioCheckData]{
		Params: nil,
		Docs:   []string{docs.KeyProduct},
		Run: func(item *template.ScenarioCheckData, input Input) (Verdict, error) {
			source, err := product.LoadCatalogue(input.Docs[docs.KeyProduct])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the catalogue: %w", err)
			}

			verdict, unchecked := activation(item, source)
			if unchecked {
				return verdict, nil
			}

			return ReviewPhrases(item.ID, item.Phrases, source), nil
		},
	}
}

// activation is the verdict of an Item every closure check leaves
// unchecked, and true, or false when the check must read the Item.
func activation(item *template.ScenarioCheckData, source product.Catalogue) (Verdict, bool) {
	switch {
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
func notChecked(subjectID, reason string) Verdict {
	return Verdict{Pass: true, Context: []string{subjectID + ": not checked: " + reason}}
}

// prefixed heads each finding with the Item's id, as a context line reads.
func prefixed(subjectID string, findings []string) []string {
	context := make([]string, 0, len(findings))
	for _, finding := range findings {
		context = append(context, subjectID+": "+finding)
	}

	return context
}

// ReviewPhrases holds one Item's steps to the catalogue: a malformed
// template fails every checked Item, since no step can be held to it.
// `build tests` holds every Registry Scenario to it the same way.
func ReviewPhrases(subjectID string, steps []catalogue.Step, source product.Catalogue) Verdict {
	compiled, malformed := catalogue.Compile(source.Entries, source.Roles)
	if len(malformed) > 0 {
		lines := make([]string, 0, len(malformed))
		for _, malformation := range malformed {
			lines = append(lines, malformation.Line())
		}

		return Verdict{Pass: false, Context: prefixed(subjectID, lines)}
	}

	review := compiled.Review(steps)
	if len(review.Findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(subjectID, review.Findings)}
	}

	return Verdict{Pass: true, Context: []string{subjectID + ": " + passLine(review)}}
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
