package checklist

import "slices"

// ConceptJudgement is the answer protocol a Q Prompt's with.concepts selects.
type ConceptJudgement string

// The concept judgements a Q Prompt may select.
const (
	JudgementLink     ConceptJudgement = "link"
	JudgementGroup    ConceptJudgement = "group"
	JudgementPropose  ConceptJudgement = "propose"
	JudgementBindings ConceptJudgement = "bindings"
	JudgementWords    ConceptJudgement = "words"
	JudgementSketch   ConceptJudgement = "sketch"
)

// Declared reports a judgement a Q Prompt may select.
func (j ConceptJudgement) Declared() bool {
	return slices.Contains([]ConceptJudgement{JudgementLink, JudgementGroup, JudgementPropose, JudgementBindings,
		JudgementWords, JudgementSketch}, j)
}

// Scored reports a judgement whose verdict a with.threshold sets.
func (j ConceptJudgement) Scored() bool {
	return j == JudgementLink || j == JudgementGroup
}

// Judgement is the concept judgement the Prompt's with.concepts names, empty when it names none as text.
func (p *Prompt) Judgement() ConceptJudgement {
	kind, _ := p.With["concepts"].(string)

	return ConceptJudgement(kind)
}
