package bootstrap

import (
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// NewModelRegistry builds the tier→model table from `engine.models`
// plus one default tier per role — `engine.default_prompt_model`,
// `engine.default_fix_model`, `engine.default_apply_model`.
//
// Validation is eager on purpose: a typo'd tier, an unknown cli, or a
// default naming an unconfigured tier fails the command here rather
// than silently substituting some other model halfway through a walk.
func NewModelRegistry(cfg *config.ViperConfig) (*provider.Registry, error) {
	defaults := make(map[provider.Role]string, len(provider.Roles()))
	for _, role := range provider.Roles() {
		defaults[role] = cfg.GetString(role.ConfigKey())
	}

	return provider.NewRegistry(cfg.GetStringMapString("engine.models"), defaults)
}
