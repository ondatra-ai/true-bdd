package validate

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// ValidateTemplates parses both templates used by this role before execution.
func (e *ChecklistEvaluator) ValidateTemplates() error {
	return validateTemplates(e.systemLoader, e.userLoader)
}

// ValidateTemplates parses both templates used by this role before execution.
func (g *FixPromptGenerator) ValidateTemplates() error {
	return validateTemplates(g.systemLoader, g.userLoader)
}

// ValidateTemplates parses both templates used by this role before execution.
func (a *FixApplier) ValidateTemplates() error {
	return validateTemplates(a.systemLoader, a.userLoader)
}

func validateTemplates[T any](system, user *template.TemplateLoader[T]) error {
	err := system.Validate()
	if err != nil {
		return fmt.Errorf("system template: %w", err)
	}

	userErr := user.Validate()
	if userErr != nil {
		return fmt.Errorf("user template: %w", userErr)
	}

	return nil
}
