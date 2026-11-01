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
	keys := slices.Sorted(maps.Keys(linked))

	answers, err := e.judgeShared(ctx, prompt, sharedJudgement{judgement: checklist.JudgementWords, keys: keys,
		request: func(key string) (any, string, error) {
			one := map[string]product.Concept{key: linked[key]}
			schema, err := wordsSchema(one)

			return struct {
				Product  string                     `json:"product"`
				Concepts map[string]product.Concept `json:"concepts"`
			}{productText, one}, schema, err
		}})
	if err != nil {
		return err
	}

	for _, key := range keys {
		gradeWords(answers[key], []string{key}, result)
	}

	return nil
}
