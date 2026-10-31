package validate

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func (e *ChecklistEvaluator) evaluateLink(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Document, result checklist.ValidationResult,
) (checklist.ValidationResult, error) {
	resetConceptLinks(subject)
	steps := conceptSteps(subject)

	if e.shares(subject, prompt) {
		handled, err := e.evaluateSharedLink(ctx, subject, prompt, source, steps, &result)
		if handled || err != nil {
			return result, err
		}
	}

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

	grade, err := gradeLinks(response, steps, source.Concepts, linkThreshold(prompt))
	e.finishLink(subject, grade, err, &result)

	return result, nil
}

// finishLink records the link's verdict on the cell and, when it passed, its links on the subject.
func (e *ChecklistEvaluator) finishLink(subject any, grade linkGrade, err error, result *checklist.ValidationResult) {
	result.Context = grade.findings
	if err != nil {
		failConcept(result, "link answer malformed or incomplete: "+err.Error())
	}

	if !grade.passed {
		result.Status = checklist.StatusFail
		result.ActualAnswer = AnswerFail
	}

	linked := make([]linkedStep, 0)

	if result.Status == checklist.StatusPass {
		storeConceptLinks(subject, grade.links, true)

		service := ""
		if item, ok := subject.(*template.ScenarioCheckData); ok {
			service = item.Service
		}

		for index, link := range grade.links {
			linked = append(linked, linkedStep{service: service, key: linkKey(link), step: grade.steps[index]})
		}
	}

	e.sharing.publish(e.subjectID, linked)
	e.saveConceptResult(*result)
}

func linkThreshold(prompt checklist.PromptWithContext) int {
	if configured, ok := prompt.Prompt.With["threshold"].(int); ok {
		return configured
	}

	return defaultLinkThreshold
}
