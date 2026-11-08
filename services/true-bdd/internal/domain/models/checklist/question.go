package checklist

import "strings"

// decodeQuestion retains invalid forms for the runner's positioned startup refusal.
func (p *Prompt) decodeQuestion(raw map[string]interface{}) {
	if _, legacy := raw["D"]; legacy {
		p.questionError = ErrLegacyQuestion

		return
	}

	question, mapping := raw["Q"].(map[string]interface{})
	if !mapping {
		p.questionError = ErrPromptHasNoKind

		return
	}

	_, prompt := question["prompt"]

	_, code := question["code"]
	switch {
	case prompt && code:
		p.questionError = ErrPromptHasBothKinds
	case !prompt && !code:
		p.questionError = ErrPromptHasNoKind
	case len(question) != 1:
		p.questionError = ErrQuestionOptions
	default:
		p.Question = getStringFromMap(question, "prompt")
		p.Check = strings.TrimSpace(getStringFromMap(question, "code"))
	}
}
