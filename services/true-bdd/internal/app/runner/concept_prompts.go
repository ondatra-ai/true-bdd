package runner

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

const linkJudgement = "link"

// Template hosts retain their original cells and request numbering.
func conceptPrompts(prompts []checklist.PromptWithContext, resolver *docs.Resolver) []checklist.PromptWithContext {
	path, err := resolver.Resolve(docs.KeyProduct)
	if err != nil {
		return withoutConceptJudgements(prompts)
	}

	source, err := product.LoadCatalogue(path)
	if err != nil || !source.ConceptsDeclared {
		return withoutConceptJudgements(prompts)
	}

	return prompts
}

func withoutConceptJudgements(prompts []checklist.PromptWithContext) []checklist.PromptWithContext {
	filtered := make([]checklist.PromptWithContext, 0, len(prompts))
	for _, prompt := range prompts {
		kind := prompt.Prompt.With["concepts"]
		if !prompt.Prompt.IsCheck() && prompt.Prompt.ShapeError() == nil &&
			(kind == linkJudgement || kind == "bindings" || kind == "words" || kind == "group" || kind == "propose") {
			continue
		}

		filtered = append(filtered, prompt)
	}

	return filtered
}
