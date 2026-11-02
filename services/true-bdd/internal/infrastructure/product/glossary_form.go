package product

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// ErrGlossaryForm refuses definitions too long to serve as concise meanings.
var ErrGlossaryForm = errors.New("glossary means must be one paragraph of at most 3 sentences, each at most 25 words")

func validateGlossaryForm(path string, node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return nil
	}

	for _, entry := range node.Content {
		var definition catalogue.Definition

		err := entry.Decode(&definition)
		if err != nil {
			continue // The command retains its existing malformed-glossary finding.
		}

		sentences, longest := glossaryCounts(definition.Means)
		paragraphs := glossaryParagraphs(definition.Means)

		const maxSentences, maxWords = 3, 25
		if sentences > maxSentences || longest > maxWords || paragraphs != 1 {
			return fmt.Errorf("product document %s: glossary term %q: %d sentences, "+
				"longest sentence %d words, %d paragraphs: %w",
				path, definition.Term, sentences, longest, paragraphs, ErrGlossaryForm)
		}
	}

	return nil
}

func glossaryCounts(text string) (int, int) {
	runes := []rune(text)
	start, sentences, longest := 0, 0, 0

	for index, char := range runes {
		terminal := strings.ContainsRune(".!?", char)
		if !terminal || (index+1 < len(runes) && !unicode.IsSpace(runes[index+1])) {
			continue
		}

		words := len(strings.Fields(string(runes[start : index+1])))
		if words > 0 {
			sentences++
			longest = max(longest, words)
		}

		start = index + 1
	}

	if words := len(strings.Fields(string(runes[start:]))); words > 0 {
		sentences++
		longest = max(longest, words)
	}

	return sentences, longest
}

func glossaryParagraphs(text string) int {
	paragraphs, inParagraph := 0, false

	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			inParagraph = false

			continue
		}

		if !inParagraph {
			paragraphs++
		}

		inParagraph = true
	}

	return paragraphs
}
