package ports

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

// AIPort defines the interface for AI communication. ModelRef names both
// the CLI and model for the turn, so a role can point at a different
// provider with no code change; Role does NOT affect dispatch — it only attributes the turn in logs.
type AIPort interface {
	ExecutePromptWithSystem(
		ctx context.Context,
		role provider.Role,
		systemPrompt string,
		userPrompt string,
		model provider.ModelRef,
		mode ai.ExecutionMode,
	) (string, error)
}
