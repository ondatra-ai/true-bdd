package checklist

import (
	"errors"
	"fmt"
)

var errConceptPrompt = errors.New("with.concepts requires a Q: link, group, propose, bindings, words or sketch; " +
	"link threshold must be an integer from 1 to 10")

var errSharedPrompt = errors.New("with.share takes true or false on a link, bindings or words Q:; " +
	"with.batch takes a positive step count on a shared link")

// defaultSharedBatch is the distinct steps one shared link turn judges, about today's steps per Scenario (#264).
const defaultSharedBatch = 10

const (
	linkKind     = "link"
	bindingsKind = "bindings"
	wordsKind    = "words"
)

// Concept parameters select the answer protocol, without making a Q a built-in D check.
func (p *Prompt) conceptShapeError() error {
	raw, present := p.With["concepts"]
	if !present {
		return nil
	}

	kind, text := raw.(string)

	valid := kind == linkKind || kind == bindingsKind || kind == "sketch" || kind == wordsKind || kind == "group" ||
		kind == "propose"
	if !text || p.IsCheck() || !valid {
		return fmt.Errorf("%s: %w", p.Label(), errConceptPrompt)
	}

	return errors.Join(p.conceptThresholdError(kind), p.shareError(kind), p.batchError(kind))
}

// SharesJudgement reports a concept Prompt that judges each step, binding or concept once per walk (#264).
func (p *Prompt) SharesJudgement() bool {
	share, _ := p.With["share"].(bool)

	return share
}

// WaitsForLinks reports a shared binding judgement, which needs every walked Scenario's link first.
func (p *Prompt) WaitsForLinks() bool {
	return p.SharesJudgement() && p.With["concepts"] == bindingsKind
}

// SharedBatch is how many distinct steps one shared link turn judges, or 0 for any other Prompt.
func (p *Prompt) SharedBatch() int {
	if !p.SharesJudgement() || p.With["concepts"] != linkKind {
		return 0
	}

	if batch, configured := p.With["batch"].(int); configured {
		return batch
	}

	return defaultSharedBatch
}

func (p *Prompt) shareError(kind string) error {
	raw, configured := p.With["share"]
	_, boolean := raw.(bool)

	if configured && (!boolean || (kind != linkKind && kind != bindingsKind && kind != wordsKind)) {
		return fmt.Errorf("%s: %w", p.Label(), errSharedPrompt)
	}

	return nil
}

func (p *Prompt) batchError(kind string) error {
	raw, configured := p.With["batch"]
	batch, integer := raw.(int)

	if configured && (!integer || batch < 1 || kind != linkKind || !p.SharesJudgement()) {
		return fmt.Errorf("%s: %w", p.Label(), errSharedPrompt)
	}

	return nil
}

func (p *Prompt) conceptThresholdError(kind string) error {
	if raw, configured := p.With["threshold"]; configured {
		threshold, integer := raw.(int)

		const highestScore = 10
		if (kind != linkKind && kind != "group") || !integer || threshold < 1 || threshold > highestScore {
			return fmt.Errorf("%s: %w", p.Label(), errConceptPrompt)
		}
	}

	return nil
}
