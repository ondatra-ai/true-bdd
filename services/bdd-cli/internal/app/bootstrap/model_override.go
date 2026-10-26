package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

var (
	errModelOverrideForm      = errors.New(`model override must be "<tier>=<cli>:<model>"`)
	errModelOverrideDuplicate = errors.New("model tier overridden twice")
)

type modelOverride struct {
	tier provider.Tier
	from provider.ModelRef
	to   provider.ModelRef
}

func resolveModelOverrides(registry *provider.Registry, overrides []string) ([]modelOverride, error) {
	changes := make([]modelOverride, 0, len(overrides))

	seen := make(map[provider.Tier]bool, len(overrides))
	for _, raw := range overrides {
		change, err := resolveModelOverride(registry, raw)
		if err != nil {
			return nil, fmt.Errorf("--model %q: %w", raw, err)
		}

		if seen[change.tier] {
			return nil, fmt.Errorf("%w: %s", errModelOverrideDuplicate, change.tier)
		}

		seen[change.tier] = true
		changes = append(changes, change)
	}

	return changes, nil
}

func resolveModelOverride(registry *provider.Registry, raw string) (modelOverride, error) {
	tierName, reference, found := strings.Cut(raw, "=")
	if !found {
		return modelOverride{}, errModelOverrideForm
	}

	tier, err := provider.ParseTier(tierName)
	if err != nil {
		return modelOverride{}, err
	}

	from, err := registry.Resolve(tier)
	if err != nil {
		return modelOverride{}, err
	}

	to, err := provider.ParseModelRef(reference)
	if err != nil {
		return modelOverride{}, err
	}

	return modelOverride{tier: tier, from: from, to: to}, nil
}
