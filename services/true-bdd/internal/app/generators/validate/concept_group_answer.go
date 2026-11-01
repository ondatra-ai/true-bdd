package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

type groupAssignment struct {
	Index         *int            `json:"index"`
	Concept       string          `json:"concept"`
	Score         *int            `json:"score"`
	RunnerUp      json.RawMessage `json:"runner_up"`
	RunnerUpScore *int            `json:"runner_up_score"`
}

func gradeGroup(response string, steps []conceptStep, concepts []product.Concept, threshold int,
) ([]template.ConceptLink, []string, error) {
	var answer struct {
		Assignments []groupAssignment `json:"assignments"`
	}

	err := strictAnswer(response, &answer)
	if err != nil {
		return nil, nil, err
	}

	byIndex, err := indexGroupAnswer(answer.Assignments, steps, concepts)
	if err != nil {
		return nil, nil, err
	}

	links := make([]template.ConceptLink, 0, len(steps))
	findings := make([]string, 0)

	for index, step := range steps {
		assignment := byIndex[index+1]

		selected := selectConceptScore(nil, assignment.Concept, *assignment.Score, threshold)
		if string(assignment.RunnerUp) != schemaNull {
			selected = selectConceptScore(selected, runnerName(assignment.RunnerUp), *assignment.RunnerUpScore, threshold)
		}

		if len(selected) != 1 {
			findings = append(findings, fmt.Sprintf("%s: %s; best %s=%d; runner-up %s=%d; threshold %d",
				step.subject(), linkVerdict(step, selected), assignment.Concept, *assignment.Score,
				runnerName(assignment.RunnerUp), *assignment.RunnerUpScore, threshold))

			continue
		}

		links = append(links, template.ConceptLink{Concept: selected[0], Block: step.Block})
	}

	return links, findings, nil
}

func validGroupingRank(answer groupAssignment, step conceptStep, concepts []product.Concept) bool {
	names := groupConceptNames(step, concepts)

	if !names[answer.Concept] || !validConceptScore(answer.Score) || !validConceptScore(answer.RunnerUpScore) {
		return false
	}

	if string(answer.RunnerUp) == schemaNull {
		return len(names) == 1 && *answer.RunnerUpScore == 0
	}

	var name string
	if json.Unmarshal(answer.RunnerUp, &name) != nil {
		return false
	}

	return names[name] && name != answer.Concept && *answer.RunnerUpScore <= *answer.Score
}

func runnerName(raw json.RawMessage) string {
	var name string
	if json.Unmarshal(raw, &name) != nil || strings.TrimSpace(name) == "" {
		return noConceptName
	}

	return name
}

func indexGroupAnswer(assignments []groupAssignment, steps []conceptStep, concepts []product.Concept,
) (map[int]groupAssignment, error) {
	byIndex := make(map[int]groupAssignment)

	for _, assignment := range assignments {
		if assignment.Index == nil || *assignment.Index < 1 || *assignment.Index > len(steps) {
			return nil, errConceptAnswer
		}

		index := *assignment.Index
		if _, duplicate := byIndex[index]; duplicate {
			return nil, errConceptAnswer
		}

		if !validGroupingRank(assignment, steps[index-1], concepts) {
			return nil, errConceptAnswer
		}

		byIndex[index] = assignment
	}

	if len(byIndex) != len(steps) {
		return nil, errConceptAnswer
	}

	return byIndex, nil
}

func groupConceptNames(step conceptStep, concepts []product.Concept) map[string]bool {
	names := make(map[string]bool)

	for _, concept := range concepts {
		if concept.Block == step.Block {
			names[concept.Name] = true
		}
	}

	return names
}
