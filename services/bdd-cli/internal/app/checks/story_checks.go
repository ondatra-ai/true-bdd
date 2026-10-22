package checks

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

// ForStories is the Set a `us refine` Prompt may name: every built-in
// check that reads one Story.
func ForStories() Set[*template.StoryRefineData] {
	return Set[*template.StoryRefineData]{
		CheckStepPhrases: storyStepPhrases(),
	}
}

// storyStepPhrases passes a Story every step of which a model does not run
// matches exactly one catalogue template, declared for the step's block.
// The Story `us refine` walks is always checked once a catalogue is declared.
func storyStepPhrases() Check[*template.StoryRefineData] {
	return Check[*template.StoryRefineData]{
		Params: nil,
		Docs:   []string{docs.KeyProduct},
		Run: func(item *template.StoryRefineData, input Input) (Verdict, error) {
			source, err := product.LoadCatalogue(input.Docs[docs.KeyProduct])
			if err != nil {
				return Verdict{}, fmt.Errorf("read the catalogue: %w", err)
			}

			if !source.Declared {
				return notChecked(item.ID, notCheckedNoCatalogue), nil
			}

			return reviewCriteria(item, source), nil
		},
	}
}

// reviewCriteria holds each acceptance criterion's steps to the catalogue,
// one context line per criterion headed by its id, as the checklist's other
// Prompts report; a malformed catalogue is said once, for the whole Story.
func reviewCriteria(item *template.StoryRefineData, source product.Catalogue) Verdict {
	_, malformed := catalogue.Compile(source.Entries, source.Roles)
	if len(malformed) > 0 || len(item.AcceptanceCriteria) == 0 {
		return ReviewPhrases(item.ID, nil, source)
	}

	verdict := Verdict{Pass: true, Context: make([]string, 0, len(item.AcceptanceCriteria))}

	for _, criterion := range item.AcceptanceCriteria {
		review := ReviewPhrases(criterion.ID, criterion.Phrases(), source)
		verdict.Pass = verdict.Pass && review.Pass
		verdict.Context = append(verdict.Context, review.Context...)
	}

	return verdict
}
