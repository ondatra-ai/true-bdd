package bootstrap

import (
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// NewModelRegistry builds the tier→model table from `engine.models`
// plus one default tier per role. Validation is eager: a typo'd tier
// or unknown cli fails here, never substitutes silently mid-walk.
func NewModelRegistry(cfg *config.ViperConfig) (*provider.Registry, error) {
	defaults := make(map[provider.Role]string, len(provider.Roles()))
	for _, role := range provider.Roles() {
		defaults[role] = cfg.GetString(role.ConfigKey())
	}

	return provider.NewRegistry(cfg.GetStringMapString("engine.models"), defaults)
}
