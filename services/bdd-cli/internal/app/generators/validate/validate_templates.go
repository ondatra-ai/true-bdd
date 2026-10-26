package validate

import "fmt"

// ValidateTemplates parses both templates used by this role before execution.
func (e *ChecklistEvaluator) ValidateTemplates() error {
	err := e.systemLoader.Validate()
	if err != nil {
		return fmt.Errorf("system template: %w", err)
	}

	err = e.userLoader.Validate()
	if err != nil {
		return fmt.Errorf("user template: %w", err)
	}

	return nil
}

// ValidateTemplates parses both templates used by this role before execution.
func (g *FixPromptGenerator) ValidateTemplates() error {
	err := g.systemLoader.Validate()
	if err != nil {
		return fmt.Errorf("system template: %w", err)
	}

	err = g.userLoader.Validate()
	if err != nil {
		return fmt.Errorf("user template: %w", err)
	}

	return nil
}

// ValidateTemplates parses both templates used by this role before execution.
func (a *FixApplier) ValidateTemplates() error {
	err := a.systemLoader.Validate()
	if err != nil {
		return fmt.Errorf("system template: %w", err)
	}

	err = a.userLoader.Validate()
	if err != nil {
		return fmt.Errorf("user template: %w", err)
	}

	return nil
}
