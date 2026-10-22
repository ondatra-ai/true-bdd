package checks

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
)

// reviewTerms holds the terms one Item's steps rely on to one term
// document, through review; held names what a pass found each term to be
// ("defined", "mapped"). A template the grammar refuses declares nothing here.
func reviewTerms(
	itemID string,
	steps []catalogue.Step,
	source product.Catalogue,
	held string,
	review func([]catalogue.Declaration) catalogue.TermReview,
) Verdict {
	compiled, _ := catalogue.Compile(source.Entries, source.Roles)

	found := review(compiled.Declarations(steps))
	if len(found.Findings) > 0 {
		return Verdict{Pass: false, Context: prefixed(itemID, found.Findings)}
	}

	if len(found.Terms) == 0 {
		return Verdict{Pass: true, Context: []string{itemID + ": no matched template declares a term"}}
	}

	return Verdict{Pass: true, Context: []string{
		itemID + ": every declared term is " + held + ": " + strings.Join(found.Terms, ", "),
	}}
}
