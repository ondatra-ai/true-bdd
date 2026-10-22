package validate

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

// roleTier is one turn a Cell may run and the raw tier its prompt names.
type roleTier struct {
	role provider.Role
	raw  string
}

// UnresolvedTiers lists, one error per turn, the tiers one prompt's Cell
// would ask for that engine.models cannot resolve: the validation turn of
// a `Q:`, and under fix the fix and apply turns of any prompt.
func (e *ChecklistEvaluator) UnresolvedTiers(promptCtx *checklist.PromptWithContext, fix bool) []error {
	wanted := make([]roleTier, 0, len(provider.Roles()))

	if !promptCtx.Prompt.IsCheck() {
		wanted = append(wanted, roleTier{role: provider.RolePrompt, raw: promptCtx.EffectiveModelTier()})
	}

	if fix {
		wanted = append(wanted,
			roleTier{role: provider.RoleFix, raw: promptCtx.EffectiveFixTier()},
			roleTier{role: provider.RoleApply, raw: promptCtx.EffectiveApplyTier()},
		)
	}

	unresolved := make([]error, 0)

	for _, want := range wanted {
		_, err := e.models.ResolveRole(want.role, want.raw)
		if err != nil {
			unresolved = append(unresolved, fmt.Errorf("%s turn: %w", want.role, err))
		}
	}

	return unresolved
}
