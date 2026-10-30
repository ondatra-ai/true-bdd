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

const stepCritiqueSchema = `{
 "type":"object", "additionalProperties":false,
 "properties": {
  "answer":{"type":"string","enum":["pass","fail"]},
  "findings":{"type":"array","items":{
   "type":"object","additionalProperties":false,
   "properties":{
    "definition":{"type":"string","minLength":1},
    "category":{"type":"string","enum":["phrase_mapping","phrase_meaning","computes_assertion"]},
    "finding":{"type":"string","minLength":1}
   },"required":["definition","category","finding"]
  }}
 },"required":["answer","findings"]
}`

// conceptCritiqueSchema names a concept Item's categories: its binding, not a Phrase mapping.
const conceptCritiqueSchema = `{
 "type":"object", "additionalProperties":false,
 "properties": {
  "answer":{"type":"string","enum":["pass","fail"]},
  "findings":{"type":"array","items":{
   "type":"object","additionalProperties":false,
   "properties":{
    "definition":{"type":"string","minLength":1},
    "category":{"type":"string","enum":["concept_binding","concept_meaning","computes_assertion"]},
    "finding":{"type":"string","minLength":1}
   },"required":["definition","category","finding"]
  }}
 },"required":["answer","findings"]
}`

// ParseStepCritique routes a valid failure to named findings and a pass to none.
func ParseStepCritique(response string) ([]string, error) {
	var answer stepCritiqueAnswer

	err := json.Unmarshal([]byte(response), &answer)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStepCritiqueAnswer, err)
	}

	if answer.Findings == nil || (answer.Answer != AnswerPass && answer.Answer != AnswerFail) ||
		(answer.Answer == AnswerPass) != (len(answer.Findings) == 0) {
		return nil, ErrStepCritiqueAnswer
	}

	findings := make([]string, 0, len(answer.Findings))
	for _, finding := range answer.Findings {
		if strings.TrimSpace(finding.Definition) == "" || strings.TrimSpace(finding.Finding) == "" ||
			!validCritiqueCategory(finding.Category) {
			return nil, ErrStepCritiqueAnswer
		}

		findings = append(findings, fmt.Sprintf("%s: %s: %s", finding.Definition, finding.Category, finding.Finding))
	}

	return findings, nil
}

func validCritiqueCategory(category string) bool {
	return category == "phrase_mapping" || category == "phrase_meaning" || category == "computes_assertion" ||
		category == "concept_binding" || category == "concept_meaning"
}
