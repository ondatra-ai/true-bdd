package bootstrap

import (
	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
)

// NewModelRegistry builds the tier→model table from `engine.models`
// and `engine.default_model`.
//
// Validation is eager on purpose: a typo'd tier, an unknown cli, or a
// default naming an unconfigured tier fails the command here rather
// than silently substituting some other model halfway through a walk.
func NewModelRegistry(cfg *config.ViperConfig) (*provider.Registry, error) {
	return provider.NewRegistry(
		cfg.GetStringMapString("engine.models"),
		cfg.GetString("engine.default_model"),
	)
}
