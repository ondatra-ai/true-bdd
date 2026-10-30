package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

func (e *ChecklistEvaluator) evaluateWords(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Document, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	links, passed := conceptLinks(subject)
	if !passed {
		failConcept(&result, "word judgement requires this walk's successful link")
		e.saveConceptResult(result)

		return result, nil
	}

	linked := make(map[string]product.Concept)
	for _, link := range links {
		linked[string(link.Block)+":"+link.Concept] = linkedConcept(source.Concepts, link)
	}

	productText, err := e.conceptProduct()
	if err != nil {
		return result, err
	}

	schema, err := wordsSchema(linked)
	if err != nil {
		return result, err
	}

	payload := struct {
		Item     string                     `json:"item"`
		Product  string                     `json:"product"`
		Concepts map[string]product.Concept `json:"concepts"`
	}{e.subjectID, productText, linked}

	response, err := e.conceptTurn(ctx, prompt, payload, schema)
	if err != nil {
		return result, err
	}

	gradeWords(response, slices.Sorted(maps.Keys(linked)), &result)
	e.saveConceptResult(result)

	return result, nil
}

func wordsSchema(linked map[string]product.Concept) (string, error) {
	properties := answerSchema{}
	for key := range linked {
		properties[key] = requiredObject(answerSchema{
			"undeclared_words": answerSchema{
				schemaType: "array", "items": answerSchema{schemaType: schemaString, schemaMinLength: 1},
				"uniqueItems": true,
			},
			schemaReason: answerSchema{schemaType: schemaString, schemaMinLength: 1},
		})
	}

	encoded, err := json.Marshal(requiredObject(answerSchema{"concepts": requiredObject(properties)}))
	if err != nil {
		return "", fmt.Errorf("encode word schema: %w", err)
	}

	return string(encoded), nil
}
