package validate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func (e *ChecklistEvaluator) evaluateGroup(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Catalogue, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	item, ok := subject.(*template.ScenarioCheckData)
	if !ok {
		return result, errConceptAnswer
	}

	resetConceptLinks(subject)
	steps := conceptSteps(subject)

	if len(steps) > 0 && !hasGroupConcept(steps[0], source.Concepts) {
		for _, step := range steps {
			failConcept(&result, step.subject()+": "+linkVerdict(step, nil))
		}

		e.saveConceptResult(result)

		return result, nil
	}

	productText, err := e.conceptProduct()
	if err != nil {
		return result, err
	}

	schema, err := groupSchema(steps, source.Concepts)
	if err != nil {
		return result, err
	}

	payload := struct {
		Suite   string        `json:"suite"`
		Service string        `json:"service"`
		Product string        `json:"product"`
		Steps   []conceptStep `json:"steps"`
	}{e.subjectID, item.Service, productText, steps}

	response, err := e.conceptTurn(ctx, prompt, payload, schema)
	if err != nil {
		return result, err
	}

	links, findings, err := gradeGroup(response, steps, source.Concepts, linkThreshold(prompt))
	if err != nil {
		return result, fmt.Errorf("grouping answer malformed or incomplete (crash): %w", err)
	}

	result.Context = findings
	if len(findings) > 0 {
		result.Status = checklist.StatusFail
		result.ActualAnswer = AnswerFail
	} else {
		storeConceptLinks(subject, links, true)
	}

	e.saveConceptResult(result)

	return result, nil
}

func groupSchema(steps []conceptStep, concepts []product.Concept) (string, error) {
	names := make([]string, 0)

	for _, concept := range concepts {
		if len(steps) > 0 && concept.Block == steps[0].Block {
			names = append(names, concept.Name)
		}
	}

	best := answerSchema{schemaType: schemaString, "enum": names}

	runner := best
	if len(names) == 1 {
		runner = answerSchema{schemaType: schemaNull}
	}

	score := answerSchema{schemaType: schemaInteger, "minimum": 0, "maximum": maxConceptScore}
	assignment := requiredObject(answerSchema{
		"index": answerSchema{schemaType: schemaInteger}, "concept": best, "score": score,
		"runner_up": runner, "runner_up_score": score})

	encoded, err := json.Marshal(requiredObject(answerSchema{
		"assignments": answerSchema{schemaType: "array", "items": assignment},
	}))
	if err != nil {
		return "", fmt.Errorf("encode grouping schema: %w", err)
	}

	return string(encoded), nil
}

func hasGroupConcept(step conceptStep, concepts []product.Concept) bool {
	for _, concept := range concepts {
		if concept.Block == step.Block {
			return true
		}
	}

	return false
}
