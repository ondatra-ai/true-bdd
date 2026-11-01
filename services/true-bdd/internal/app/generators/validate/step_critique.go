package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrStepCritiqueAnswer identifies an unusable critique, never a passing verdict.
var ErrStepCritiqueAnswer = errors.New("unusable step critique answer")

type stepCritiqueAnswer struct {
	Answer   string                `json:"answer"`
	Findings []stepCritiqueFinding `json:"findings"`
}

type stepCritiqueFinding struct {
	Definition string `json:"definition"`
	Category   string `json:"category"`
	Finding    string `json:"finding"`
}

// stepCritiqueSchema names a concept Item's categories: its patterns, which
// an AI proposed, and its bodies against the concept's binding.
const stepCritiqueSchema = `{
 "type":"object", "additionalProperties":false,
 "properties": {
  "answer":{"type":"string","enum":["pass","fail"]},
  "findings":{"type":"array","items":{
   "type":"object","additionalProperties":false,
   "properties":{
    "definition":{"type":"string","minLength":1},
    "category":{"type":"string","enum":["pattern_captures","pattern_scope",
     "concept_binding","concept_meaning","computes_assertion"]},
    "finding":{"type":"string","minLength":1}
   },"required":["definition","category","finding"]
  }}
 },"required":["answer","findings"]
}`

// The categories of a finding against a concept definition's pattern.
const (
	critiquePatternCaptures = "pattern_captures"
	critiquePatternScope    = "pattern_scope"
)

// CritiqueFinding is one discrepancy the critique names in one definition.
type CritiqueFinding struct {
	Definition string `json:"definition"`
	Category   string `json:"category"`
	Finding    string `json:"finding"`
}

// Pattern reports a finding against the definition's pattern, which sends
// a concept Item back to its proposal; any other finding is against the
// body, which goes back to the writer.
func (f CritiqueFinding) Pattern() bool {
	return f.Category == critiquePatternCaptures || f.Category == critiquePatternScope
}

func (f CritiqueFinding) String() string {
	return fmt.Sprintf("%s: %s: %s", f.Definition, f.Category, f.Finding)
}

// ParseStepCritique routes a valid failure to named findings and a pass to none.
func ParseStepCritique(response string) ([]CritiqueFinding, error) {
	var answer stepCritiqueAnswer

	err := json.Unmarshal([]byte(response), &answer)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStepCritiqueAnswer, err)
	}

	if answer.Findings == nil || (answer.Answer != AnswerPass && answer.Answer != AnswerFail) ||
		(answer.Answer == AnswerPass) != (len(answer.Findings) == 0) {
		return nil, ErrStepCritiqueAnswer
	}

	findings := make([]CritiqueFinding, 0, len(answer.Findings))
	for _, finding := range answer.Findings {
		if strings.TrimSpace(finding.Definition) == "" || strings.TrimSpace(finding.Finding) == "" ||
			!validCritiqueCategory(finding.Category) {
			return nil, ErrStepCritiqueAnswer
		}

		findings = append(findings, CritiqueFinding(finding))
	}

	return findings, nil
}

func validCritiqueCategory(category string) bool {
	return category == "computes_assertion" || category == "concept_binding" || category == "concept_meaning" ||
		category == critiquePatternCaptures || category == critiquePatternScope
}
