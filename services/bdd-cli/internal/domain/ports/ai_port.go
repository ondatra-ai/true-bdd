package ports

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

// AIPort defines the interface for AI communication. ModelRef names both
// the CLI and model for the turn, so a role can point at a different
// provider with no code change; Role attributes telemetry and confines execution options to their role.
type AIPort interface {
	ExecutePromptWithSystem(
		ctx context.Context,
		role provider.Role,
		systemPrompt string,
		userPrompt string,
		model provider.ModelRef,
		mode provider.ExecutionMode,
		// resultSchema empty leaves the turn unconstrained; set, the
		// answer IS that schema's object.
		resultSchema string,
		options ...provider.TurnOptions,
	) (string, error)
}
