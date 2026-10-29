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

func gradeLinks(response string, steps []conceptStep, concepts []product.Concept, threshold int,
) ([]template.ConceptLink, []string, bool, error) {
	var answer struct {
		Steps map[string]stepScores `json:"steps"`
	}

	err := strictAnswer(response, &answer)
	if err != nil {
		return nil, nil, false, err
	}

	if answer.Steps == nil {
		return nil, nil, false, errConceptAnswer
	}

	links := make([]template.ConceptLink, 0)
	findings := make([]string, 0, len(steps))
	passed := true

	for _, step := range steps {
		if step.judged() {
			findings = append(findings, step.subject()+": judge: unlinked")

			continue
		}

		scores, ok := answer.Steps[step.Key]
		if !ok {
			return nil, findings, false, fmt.Errorf("step %s missing: %w", step.Key, errConceptAnswer)
		}

		delete(answer.Steps, step.Key)

		selected, err := scoredConcepts(step, scores, concepts, threshold)
		if err != nil {
			return nil, findings, false, err
		}

		if string(scores.Other) != "null" {
			selected = nil
		}

		verdict := linkVerdict(step, selected)
		other := otherConceptText(scores.Other, step, concepts)
		findings = append(findings, fmt.Sprintf("%s: %s; threshold %d; scores %s; reason: %s; other block: %s",
			step.subject(), verdict, threshold, scoreText(scores.Scores), scores.Reason, other))

		if len(selected) != 1 {
			passed = false

			continue
		}

		links = append(links, template.ConceptLink{Concept: selected[0], Block: step.Block,
			Intent: strings.HasPrefix(strings.TrimSpace(step.Text), "llm:")})
	}

	if len(answer.Steps) != 0 {
		return nil, findings, false, errConceptAnswer
	}

	return links, findings, passed, nil
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

		if *score >= threshold {
			selected = append(selected, concept.Name)
		}
	}

	if len(scores.Scores) != count || strings.TrimSpace(scores.Reason) == "" || !validOther(scores.Other, step, concepts) {
		return nil, fmt.Errorf("step %s scores, reason or other_block_concept: %w", step.Key, errConceptAnswer)
	}

	return selected, nil
}

func validOther(raw json.RawMessage, step conceptStep, concepts []product.Concept) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
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
		return "none"
	}

	names := make([]string, 0)

	for _, concept := range concepts {
		if concept.Block != step.Block && concept.Name == name {
			names = append(names, fmt.Sprintf("%s concept %q", concept.Block, concept.Name))
		}
	}

	return strings.Join(names, ", ")
}
