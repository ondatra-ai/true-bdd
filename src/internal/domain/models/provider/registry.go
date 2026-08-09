package provider

import (
	"errors"
	"fmt"
	"strings"
)

// Registry configuration failures — all raised at startup.
var (
	ErrNoModelsConfigured = errors.New("engine.models is empty: configure at least one tier")
	ErrTierNotConfigured  = errors.New("model tier is not configured under engine.models")
	ErrDefaultTierMissing = errors.New("default model tier names a tier that engine.models does not configure")
)

// Registry resolves a Tier to the ModelRef that runs it.
//
// Built once at startup from `engine.models` plus one default tier per
// Role, and validated eagerly, so a typo'd tier fails the command up
// front rather than silently downgrading a turn halfway through a walk.
type Registry struct {
	byTier       map[Tier]ModelRef
	defaultTiers map[Role]Tier
}

// NewRegistry validates the raw `engine.models` map (tier name →
// `"<cli>:<model>"`) together with each role's configured default tier
// (role → tier name, from `engine.default_<role>_model`).
func NewRegistry(rawModels map[string]string, rawDefaults map[Role]string) (*Registry, error) {
	if len(rawModels) == 0 {
		return nil, ErrNoModelsConfigured
	}

	byTier := make(map[Tier]ModelRef, len(rawModels))

	for rawTier, rawRef := range rawModels {
		tier, err := ParseTier(rawTier)
		if err != nil {
			return nil, fmt.Errorf("engine.models: %w", err)
		}

		ref, err := ParseModelRef(rawRef)
		if err != nil {
			return nil, fmt.Errorf("engine.models.%s: %w", rawTier, err)
		}

		byTier[tier] = ref
	}

	defaultTiers, err := parseDefaultTiers(byTier, rawDefaults)
	if err != nil {
		return nil, err
	}

	return &Registry{byTier: byTier, defaultTiers: defaultTiers}, nil
}

// parseDefaultTiers validates every role's default in Roles() order, so
// a config with several bad keys always reports the same one first.
func parseDefaultTiers(byTier map[Tier]ModelRef, rawDefaults map[Role]string) (map[Role]Tier, error) {
	defaultTiers := make(map[Role]Tier, len(Roles()))

	for _, role := range Roles() {
		raw := rawDefaults[role]

		tier, err := ParseTier(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", role.ConfigKey(), err)
		}

		if _, ok := byTier[tier]; !ok {
			return nil, fmt.Errorf("%s: %w: %q", role.ConfigKey(), ErrDefaultTierMissing, raw)
		}

		defaultTiers[role] = tier
	}

	return defaultTiers, nil
}

// DefaultTier is the tier a role falls back to when neither a prompt nor
// its checklist names one.
func (r *Registry) DefaultTier(role Role) Tier {
	return r.defaultTiers[role]
}

// Resolve maps a tier to its ModelRef. The tier must be a real one —
// "the checklist said nothing" is ResolveRole's business, because which
// default applies depends on the role asking.
func (r *Registry) Resolve(tier Tier) (ModelRef, error) {
	ref, ok := r.byTier[tier]
	if !ok {
		return ModelRef{}, fmt.Errorf("%w: %q (configured: %s)",
			ErrTierNotConfigured, tier, joinTiers(r.configuredTiers()))
	}

	return ref, nil
}

// ResolveRole validates and resolves a raw tier name straight from
// YAML — a checklist's `prompt_model:` or a prompt's `model:` — for the
// turn that is about to run. The empty string means the role's default,
// i.e. `engine.default_<role>_model`.
func (r *Registry) ResolveRole(role Role, raw string) (ModelRef, error) {
	if strings.TrimSpace(raw) == "" {
		return r.Resolve(r.DefaultTier(role))
	}

	tier, err := ParseTier(raw)
	if err != nil {
		return ModelRef{}, err
	}

	return r.Resolve(tier)
}

// configuredTiers lists the tiers this registry actually holds, in the
// canonical Tiers() order so error messages are deterministic.
func (r *Registry) configuredTiers() []Tier {
	configured := make([]Tier, 0, len(r.byTier))

	for _, tier := range Tiers() {
		if _, ok := r.byTier[tier]; ok {
			configured = append(configured, tier)
		}
	}

	return configured
}
