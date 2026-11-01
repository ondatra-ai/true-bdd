package validate

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// StepCritiqueData embeds the whole specification context and steps package.
type StepCritiqueData struct {
	Definition string
	Context    string
	Sources    string
}

// StepCritic reads an applied definition separately from the writer.
type StepCritic struct {
	ai           ports.AIPort
	models       *provider.Registry
	system, user *template.TemplateLoader[StepCritiqueData]
}

// NewStepCritic uses only project templates and the prompt role's default tier.
func NewStepCritic(ai ports.AIPort, models *provider.Registry, system, user string) *StepCritic {
	return &StepCritic{ai: ai, models: models,
		system: template.NewTemplateLoader[StepCritiqueData](system),
		user:   template.NewTemplateLoader[StepCritiqueData](user),
	}
}

// Validate checks project templates and the checklist's optional tier before work.
func (c *StepCritic) Validate(tier string) error {
	_, err := c.models.ResolveRole(provider.RolePrompt, tier)
	if err != nil {
		return fmt.Errorf("critique model: %w", err)
	}

	return validateTemplates(c.system, c.user)
}

// Critique returns findings for the Fix loop; unusable replies stop the walk.
func (c *StepCritic) Critique(ctx context.Context, data StepCritiqueData, tier, artifact string,
) ([]CritiqueFinding, error) {
	model, err := c.models.ResolveRole(provider.RolePrompt, tier)
	if err != nil {
		return nil, fmt.Errorf("critique model: %w", err)
	}

	system, err := c.system.LoadTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("critique system template: %w", err)
	}

	user, err := c.user.LoadTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("critique user template: %w", err)
	}

	err = saveCritique(artifact, "system", system)
	if err != nil {
		return nil, err
	}

	err = saveCritique(artifact, "user", user)
	if err != nil {
		return nil, err
	}

	mode := provider.ExecutionMode{DisallowedTools: []string{
		"Bash", "Write", "Edit", "MultiEdit", "NotebookEdit", "Read", "Glob", "Grep",
	}}

	response, err := c.ai.ExecutePromptWithSystem(ctx, provider.RolePrompt, system, user, model, mode,
		stepCritiqueSchema)
	if err != nil {
		return nil, fmt.Errorf("step critique: %w", err)
	}

	err = saveCritique(artifact, "response", response)
	if err != nil {
		return nil, err
	}

	return ParseStepCritique(response)
}

func saveCritique(artifact, kind, content string) error {
	err := disk.Write(artifact+"-"+kind+".txt", []byte(content), disk.Private)
	if err != nil {
		return fmt.Errorf("save step critique: %w", err)
	}

	return nil
}
