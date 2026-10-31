package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

type conceptStep struct {
	Key       string          `json:"key"`
	Criterion string          `json:"criterion,omitempty"`
	Block     catalogue.Block `json:"block"`
	Text      string          `json:"text"`
}

func conceptSteps(subject any) []conceptStep {
	steps := make([]conceptStep, 0)

	switch item := subject.(type) {
	case *template.ScenarioCheckData:
		for _, step := range item.CheckedSteps {
			steps = appendConceptStep(steps, "", step)
		}
	case *template.StoryRefineData:
		for _, criterion := range item.AcceptanceCriteria {
			for _, step := range criterion.CheckedSteps() {
				steps = appendConceptStep(steps, criterion.ID, step)
			}
		}
	}

	return steps
}

func appendConceptStep(steps []conceptStep, criterion string, step catalogue.Step) []conceptStep {
	return append(steps, conceptStep{Key: strconv.Itoa(len(steps) + 1),
		Criterion: criterion, Block: step.Block, Text: step.Text})
}

func (step conceptStep) judged() bool {
	return strings.HasPrefix(strings.TrimSpace(step.Text), "judge:")
}

func (step conceptStep) subject() string {
	return fmt.Sprintf("%s %s step %q", step.Criterion, step.Block, step.Text)
}

func conceptItemTitle(subject any) string {
	switch item := subject.(type) {
	case *template.StoryRefineData:
		return item.Title
	case *template.ScenarioCheckData:
		return item.Description
	default:
		return ""
	}
}
