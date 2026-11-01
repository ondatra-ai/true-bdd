package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const noConceptName = "none"

var errConceptAnswer = errors.New("invalid concept answer")

type stepScores struct {
	Scores map[string]*int `json:"scores"`
	Reason string          `json:"reason"`
	Other  json.RawMessage `json:"other_block_concept"`
}

func strictAnswer(response string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(response))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(target)
	if err != nil {
		return fmt.Errorf("%w: %w", errConceptAnswer, err)
	}

	err = decoder.Decode(new(any))
	if !errors.Is(err, io.EOF) {
		return errConceptAnswer
	}

	return nil
}

// linkGrade is a link's verdict: each linked step beside its link, every step's finding, and whether all linked.
type linkGrade struct {
	links    []template.ConceptLink
	steps    []conceptStep
	findings []string
	passed   bool
}

func gradeLinks(response string, steps []conceptStep, concepts []product.Concept, threshold int,
) (linkGrade, error) {
	var answer struct {
		Steps map[string]stepScores `json:"steps"`
	}

	err := strictAnswer(response, &answer)
	if err != nil {
		return linkGrade{}, err
	}

	if answer.Steps == nil {
		return linkGrade{}, errConceptAnswer
	}

	grade, err := gradeStepAnswers(answer.Steps, steps, concepts, threshold)
	if err != nil {
		return grade, err
	}

	if len(answer.Steps) != 0 {
		return linkGrade{findings: grade.findings}, errConceptAnswer
	}

	return grade, nil
}

// gradeStepAnswers grades each step's scores, consuming them from answers, against the threshold.
func gradeStepAnswers(answers map[string]stepScores, steps []conceptStep, concepts []product.Concept,
	threshold int,
) (linkGrade, error) {
	grade := linkGrade{links: make([]template.ConceptLink, 0), findings: make([]string, 0, len(steps)), passed: true}

	for _, step := range steps {
		if step.judged() {
			grade.findings = append(grade.findings, step.subject()+": judge: unlinked")

			continue
		}

		scores, ok := answers[step.Key]
		if !ok {
			return linkGrade{findings: grade.findings}, fmt.Errorf("step %s missing: %w", step.Key, errConceptAnswer)
		}

		delete(answers, step.Key)

		selected, err := scoredConcepts(step, scores, concepts, threshold)
		if err != nil {
			return linkGrade{findings: grade.findings}, err
		}

		if string(scores.Other) != schemaNull {
			selected = nil
		}

		grade.findings = append(grade.findings, fmt.Sprintf("%s: %s; threshold %d; scores %s; reason: %s; other block: %s",
			step.subject(), linkVerdict(step, selected), threshold, scoreText(scores.Scores), scores.Reason,
			otherConceptText(scores.Other, step, concepts)))

		if len(selected) != 1 {
			grade.passed = false

			continue
		}

		grade.links = append(grade.links, template.ConceptLink{Concept: selected[0], Block: step.Block,
			Intent: strings.HasPrefix(strings.TrimSpace(step.Text), "llm:")})
		grade.steps = append(grade.steps, step)
	}

	return grade, nil
}

func scoredConcepts(step conceptStep, scores stepScores, concepts []product.Concept, threshold int) ([]string, error) {
	selected := make([]string, 0)
	count := 0

	for _, concept := range concepts {
		if concept.Block != step.Block {
			continue
		}

		count++

		score, ok := scores.Scores[concept.Name]
		if !ok || !validConceptScore(score) {
			return nil, fmt.Errorf("%s concept %q unscored or invalid: %w", step.Block, concept.Name, errConceptAnswer)
		}

		selected = selectConceptScore(selected, concept.Name, *score, threshold)
	}

	if len(scores.Scores) != count || strings.TrimSpace(scores.Reason) == "" || !validOther(scores.Other, step, concepts) {
		return nil, fmt.Errorf("step %s scores, reason or other_block_concept: %w", step.Key, errConceptAnswer)
	}

	return selected, nil
}

func validOther(raw json.RawMessage, step conceptStep, concepts []product.Concept) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte(schemaNull)) {
		return true
	}

	var name string
	if json.Unmarshal(raw, &name) != nil || name == "" {
		return false
	}

	for _, concept := range concepts {
		if concept.Block != step.Block && concept.Name == name {
			return true
		}
	}

	return false
}

func linkVerdict(step conceptStep, selected []string) string {
	switch len(selected) {
	case 0:
		return "no concept of " + string(step.Block) + " reaches the threshold"
	case 1:
		return "linked to " + selected[0]
	default:
		return "ambiguous: " + strings.Join(selected, ", ")
	}
}

func validConceptScore(score *int) bool {
	return score != nil && *score >= 0 && *score <= maxConceptScore
}

func scoreText(scores map[string]*int) string {
	values := make([]string, 0, len(scores))
	for _, key := range slices.Sorted(maps.Keys(scores)) {
		values = append(values, fmt.Sprintf("%q:%d", key, *scores[key]))
	}

	return "{" + strings.Join(values, ",") + "}"
}

func otherConceptText(raw json.RawMessage, step conceptStep, concepts []product.Concept) string {
	var name string
	if json.Unmarshal(raw, &name) != nil || name == "" {
		return noConceptName
	}

	names := make([]string, 0)

	for _, concept := range concepts {
		if concept.Block != step.Block && concept.Name == name {
			names = append(names, fmt.Sprintf("%s concept %q", concept.Block, concept.Name))
		}
	}

	return strings.Join(names, ", ")
}

// Both link protocols apply this threshold before deciding whether exactly one concept fits.
func selectConceptScore(selected []string, name string, score, threshold int) []string {
	if score >= threshold {
		return append(selected, name)
	}

	return selected
}
