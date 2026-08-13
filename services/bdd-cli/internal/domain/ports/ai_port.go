package ports

import (
	"context"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

// AIPort defines the interface for AI communication
// This port interface represents the contract for AI operations in the domain
// layer.
//
// The ModelRef names both the CLI that runs the turn and the model it
// runs, so a checklist role can be pointed at a different provider
// without any code change. The Role names which of the three checklist
// turns this is — it does not affect dispatch (the ModelRef already
// decided that), it attributes the turn in logs and telemetry.
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
