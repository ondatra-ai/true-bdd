package story

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

type AcceptanceCriterion struct {
	ID          string         `json:"id"              yaml:"id"`
	Description string         `json:"description"     yaml:"description"`
	Steps       []ScenarioStep `json:"steps,omitempty" yaml:"steps,omitempty"`
}

// CheckedSteps are the criterion's steps as the closure checks read them: each
// one's own text, without its and:/but: keyword, beside its block.
func (c AcceptanceCriterion) CheckedSteps() []catalogue.Step {
	steps := make([]catalogue.Step, 0, len(c.Steps))

	for _, step := range c.Steps {
		steps = appendSteps(steps, catalogue.BlockGiven, step.Given)
		steps = appendSteps(steps, catalogue.BlockWhen, step.When)
		steps = appendSteps(steps, catalogue.BlockThen, step.Then)
	}

	return steps
}

// appendSteps adds one block's statements, in the order the Story lists them.
func appendSteps(steps []catalogue.Step, block catalogue.Block, statements []StepStatement) []catalogue.Step {
	for _, statement := range statements {
		steps = append(steps, catalogue.NewStep(block, statement.Statement))
	}

	return steps
}

// UnmarshalYAML adds the criterion's id to a malformed step's refusal.
func (c *AcceptanceCriterion) UnmarshalYAML(node *yaml.Node) error {
	var identity struct {
		ID string `yaml:"id"`
	}

	err := node.Decode(&identity)
	if err != nil {
		return fmt.Errorf("acceptance criterion id: %w", err)
	}

	type plain AcceptanceCriterion

	var criterion plain

	err = node.Decode(&criterion)
	if err != nil {
		return fmt.Errorf("acceptance criterion %s: %w", identity.ID, err)
	}

	*c = AcceptanceCriterion(criterion)

	return nil
}
