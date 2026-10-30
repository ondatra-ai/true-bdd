package story

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

type AcceptanceCriterion struct {
	ID          string         `json:"id"              yaml:"id"`
	Description string         `json:"description"     yaml:"description"`
	Steps       []ScenarioStep `json:"steps,omitempty" yaml:"steps,omitempty"`
}

// CheckedSteps are the criterion's steps as the closure checks read them: each
// one's own text, without its and:/but: keyword, beside its block.
func (c AcceptanceCriterion) CheckedSteps() []catalogue.Step {
	phrases := make([]catalogue.Step, 0, len(c.Steps))

	for _, step := range c.Steps {
		phrases = appendPhrases(phrases, catalogue.BlockGiven, step.Given)
		phrases = appendPhrases(phrases, catalogue.BlockWhen, step.When)
		phrases = appendPhrases(phrases, catalogue.BlockThen, step.Then)
	}

	return phrases
}

// appendPhrases adds one block's statements, in the order the Story lists them.
func appendPhrases(phrases []catalogue.Step, block catalogue.Block, statements []StepStatement) []catalogue.Step {
	for _, statement := range statements {
		phrases = append(phrases, catalogue.NewStep(block, statement.Statement))
	}

	return phrases
}
