package checks

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// Why a Scenario passes the step-phrase check unchecked.
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

			if !source.Declared {
				return notChecked(item.ID, notCheckedNoCatalogue), nil
			}

			if len(item.UserStories) == 0 {
				return notChecked(item.ID, notCheckedUnclaimed), nil
			}

			return reviewPhrases(item.ID, item.Phrases, source), nil
		},
	}
}

// notChecked passes an Item the activation contract leaves unchecked,
// saying why, so a pass that checked nothing never reads as one that did.
func notChecked(subjectID, reason string) Verdict {
	return Verdict{Pass: true, Context: []string{subjectID + ": not checked: " + reason}}
}

// reviewPhrases holds one Item's steps to the catalogue: a malformed
// template fails every checked Item, since no step can be held to it.
func reviewPhrases(subjectID string, steps []catalogue.Step, source product.Catalogue) Verdict {
	compiled, malformed := catalogue.Compile(source.Entries, source.Roles)
	if len(malformed) > 0 {
		context := make([]string, 0, len(malformed))
		for _, malformation := range malformed {
			context = append(context, subjectID+": "+malformation.Line())
		}

		return Verdict{Pass: false, Context: context}
	}

	review := compiled.Review(steps)
	if len(review.Findings) > 0 {
		context := make([]string, 0, len(review.Findings))
		for _, finding := range review.Findings {
			context = append(context, subjectID+": "+finding)
		}

		return Verdict{Pass: false, Context: context}
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
