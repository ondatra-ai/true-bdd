package checklist

import (
	"errors"
	"fmt"
)

var errConceptPrompt = errors.New("with.concepts requires a Q: link, group, propose, bindings, words or sketch; " +
	"link threshold must be an integer from 1 to 10")

// Concept parameters select the answer protocol, without making a Q a built-in D check.
func (p *Prompt) conceptShapeError() error {
	raw, present := p.With["concepts"]
	if !present {
		return nil
	}

	kind, text := raw.(string)

	valid := kind == "link" || kind == "bindings" || kind == "sketch" || kind == "words" || kind == "group" ||
		kind == "propose"
	if !text || p.IsCheck() || !valid {
		return fmt.Errorf("%s: %w", p.Label(), errConceptPrompt)
	}

	return p.conceptThresholdError(kind)
}

func (p *Prompt) conceptThresholdError(kind string) error {
	if raw, configured := p.With["threshold"]; configured {
		threshold, integer := raw.(int)

		const highestScore = 10
		if (kind != "link" && kind != "group") || !integer || threshold < 1 || threshold > highestScore {
			return fmt.Errorf("%s: %w", p.Label(), errConceptPrompt)
		}
	}

	return nil
}
