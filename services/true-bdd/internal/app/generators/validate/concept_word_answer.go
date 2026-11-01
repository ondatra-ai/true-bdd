package validate

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
)

type conceptWordAnswer struct {
	UndeclaredWords *[]string `json:"undeclared_words"`
	Reason          string    `json:"reason"`
}

func gradeWords(response string, keys []string, result *checklist.ValidationResult) {
	var answer struct {
		Concepts map[string]conceptWordAnswer `json:"concepts"`
	}

	err := strictAnswer(response, &answer)
	if err != nil || answer.Concepts == nil {
		failConcept(result, "word answer malformed or incomplete")

		return
	}

	for _, key := range keys {
		verdict, ok := answer.Concepts[key]
		delete(answer.Concepts, key)

		if !ok || verdict.UndeclaredWords == nil || strings.TrimSpace(verdict.Reason) == "" {
			failConcept(result, key+": word answer missing or incomplete")

			continue
		}

		recordWordVerdict(key, verdict, result)
	}

	if len(answer.Concepts) > 0 {
		failConcept(result, "word answer contains unknown concepts")
	}
}

func recordWordVerdict(key string, verdict conceptWordAnswer, result *checklist.ValidationResult) {
	result.Context = append(result.Context, key+": "+verdict.Reason)

	words := *verdict.UndeclaredWords
	if len(words) == 0 {
		result.Context = append(result.Context, key+": declared terms and plain English only")

		return
	}

	for _, word := range words {
		if strings.TrimSpace(word) == "" {
			failConcept(result, key+": word answer contains an empty word")
		} else {
			failConcept(result, key+": undeclared domain word "+word)
		}
	}
}
