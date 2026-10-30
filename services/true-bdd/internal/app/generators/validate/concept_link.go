package validate

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

func (e *ChecklistEvaluator) evaluateLink(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Catalogue, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	resetConceptLinks(subject)
	steps := conceptSteps(subject)

	productText, err := e.conceptProduct()
	if err != nil {
		return result, err
	}

	payload := struct {
		Item        string        `json:"item"`
		Description string        `json:"description"`
		Product     string        `json:"product"`
		Steps       []conceptStep `json:"steps"`
	}{e.subjectID, conceptItemTitle(subject), productText, steps}

	schema, err := linkSchema(steps, source.Concepts)
	if err != nil {
		return result, err
	}

	response, err := e.conceptTurn(ctx, prompt, payload, schema)
	if err != nil {
		return result, err
	}

	threshold := defaultLinkThreshold
	if configured, ok := prompt.Prompt.With["threshold"].(int); ok {
		threshold = configured
	}

	links, findings, passed, err := gradeLinks(response, steps, source.Concepts, threshold)

	result.Context = findings
	if err != nil {
		failConcept(&result, "link answer malformed or incomplete: "+err.Error())
	}

	if !passed {
		result.Status = checklist.StatusFail
		result.ActualAnswer = AnswerFail
	}

	if result.Status == checklist.StatusPass {
		storeConceptLinks(subject, links, true)
	}

	e.saveConceptResult(result)

	return result, nil
}
