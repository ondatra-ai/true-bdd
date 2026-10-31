package validate

import (
	"context"
	"maps"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// judgeSharedWords gives the Scenario each linked concept's verdict, judged once per walk from its means alone.
func (e *ChecklistEvaluator) judgeSharedWords(ctx context.Context, prompt checklist.PromptWithContext,
	linked map[string]product.Concept, productText string, result *checklist.ValidationResult,
) error {
	names := make([]string, 0, len(linked))
	keys := make(map[string]string, len(linked))

	for _, key := range slices.Sorted(maps.Keys(linked)) {
		name := sharedKey(prompt, "words", key)
		names = append(names, name)
		keys[name] = key
	}

	answers, err := e.sharing.resolve(ctx, names, func(name string) (string, error) {
		one := map[string]product.Concept{keys[name]: linked[keys[name]]}

		schema, err := wordsSchema(one)
		if err != nil {
			return "", err
		}

		payload := struct {
			Product  string                     `json:"product"`
			Concepts map[string]product.Concept `json:"concepts"`
		}{productText, one}

		return e.sharedTurn(ctx, prompt, name, payload, schema)
	})
	if err != nil {
		return err
	}

	for _, name := range names {
		gradeWords(answers[name], []string{keys[name]}, result)
	}

	return nil
}
