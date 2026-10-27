package bootstrap

import (
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
)

// NewModelRegistry builds the tier→model table from `engine.models`
// plus one default tier per role. Validation is eager: a typo'd tier
// or unknown cli fails here, never substitutes silently mid-walk.
func NewModelRegistry(cfg *config.ViperConfig, overrides ...string) (*provider.Registry, error) {
	defaults := make(map[provider.Role]string, len(provider.Roles()))
	for _, role := range provider.Roles() {
		defaults[role] = cfg.GetString(role.ConfigKey())
	}

	models := cfg.GetStringMapString("engine.models")

	registry, err := provider.NewRegistry(models, defaults)
	if err != nil {
		return nil, err
	}

	changes, err := resolveModelOverrides(registry, overrides)
	if err != nil {
		return nil, err
	}

	for _, change := range changes {
		models[string(change.tier)] = change.to.String()
	}

	resolved, err := provider.NewRegistry(models, defaults)
	if err != nil {
		return nil, err
	}

	for _, change := range changes {
		slog.Info("Model tier overridden",
			"tier", string(change.tier), "from", change.from.String(), "to", change.to.String())
	}

	return resolved, nil
}
