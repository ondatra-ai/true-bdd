package validate

import (
	"context"
	"fmt"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// evaluateSharedLink grades the Scenario's steps from the walk's batches, each distinct step judged once with the
// product document and its Scenarios' descriptions, no sibling step. False leaves an unplanned step to today's turn.
func (e *ChecklistEvaluator) evaluateSharedLink(ctx context.Context, subject any, prompt checklist.PromptWithContext,
	source product.Document, steps []conceptStep, result *checklist.ValidationResult,
) (bool, error) {
	plan := e.sharing.linkPlan(prompt.Index, prompt.Prompt.SharedBatch())

	numbers, planned := plan.batchesFor(steps)
	if !planned {
		return false, nil
	}

	productText, err := e.conceptProduct()
	if err != nil {
		return true, err
	}

	names := make([]string, 0, len(numbers))
	for _, number := range numbers {
		names = append(names, sharedKey(prompt, "link", fmt.Sprintf("batch-%03d", number+1)))
	}

	answers, err := e.sharing.resolve(ctx, names, func(name string) (string, error) {
		number := numbers[slices.Index(names, name)]

		schema, err := linkSchema(plan.batches[number], source.Concepts)
		if err != nil {
			return "", err
		}

		payload := struct {
			Product string           `json:"product"`
			Steps   []sharedLinkStep `json:"steps"`
		}{productText, plan.batchSteps(number)}

		return e.sharedTurn(ctx, prompt, name, payload, schema)
	})
	if err != nil {
		return true, err
	}

	verdicts, malformed := batchVerdicts(answers, names, numbers, plan)
	own := make(map[string]stepScores)

	for _, step := range steps {
		if verdict, judged := verdicts[step.identity()]; judged {
			own[step.Key] = verdict
		}
	}

	grade, err := gradeStepAnswers(own, steps, source.Concepts, linkThreshold(prompt))
	if malformed != nil {
		err = malformed
	}

	e.finishLink(subject, grade, err, result)

	return true, nil
}

// batchVerdicts reads each batch's answer into verdicts by step identity; a batch answer that does not read
// fails every Scenario holding one of its steps.
func batchVerdicts(answers map[string]string, names []string, numbers []int, plan linkPlan,
) (map[string]stepScores, error) {
	verdicts := make(map[string]stepScores)

	var malformed error

	for index, name := range names {
		var answer struct {
			Steps map[string]stepScores `json:"steps"`
		}

		err := strictAnswer(answers[name], &answer)
		if err == nil && answer.Steps == nil {
			err = errConceptAnswer
		}

		batch := plan.batches[numbers[index]]
		for _, step := range batch {
			verdict, ok := answer.Steps[step.Key]
			if ok {
				verdicts[step.identity()] = verdict
			}

			delete(answer.Steps, step.Key)
		}

		if err == nil && len(answer.Steps) != 0 {
			err = errConceptAnswer
		}

		if err != nil {
			malformed = fmt.Errorf("%s: %w", name, err)
		}
	}

	return verdicts, malformed
}
