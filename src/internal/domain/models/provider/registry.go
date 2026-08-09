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
	ErrDefaultTierMissing = errors.New("engine.default_model names a tier that engine.models does not configure")
)

// Registry resolves a Tier to the ModelRef that runs it.
//
// Built once at startup from `engine.models` + `engine.default_model`
// and validated eagerly, so a typo'd tier fails the command up front
// rather than silently downgrading a turn halfway through a walk.
type Registry struct {
	byTier      map[Tier]ModelRef
	defaultTier Tier
}

// NewRegistry validates the raw `engine.models` map (tier name →
// `"<cli>:<model>"`) together with the configured default tier.
func NewRegistry(rawModels map[string]string, rawDefault string) (*Registry, error) {
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

	defaultTier, err := ParseTier(rawDefault)
	if err != nil {
		return nil, fmt.Errorf("engine.default_model: %w", err)
	}

	if _, ok := byTier[defaultTier]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrDefaultTierMissing, rawDefault)
	}

	return &Registry{byTier: byTier, defaultTier: defaultTier}, nil
}

// DefaultTier is the tier used when neither a prompt nor its checklist
// names one.
func (r *Registry) DefaultTier() Tier {
	return r.defaultTier
}

// Resolve maps a tier to its ModelRef. The empty tier means "whatever
// engine.default_model says".
func (r *Registry) Resolve(tier Tier) (ModelRef, error) {
	if tier == "" {
		tier = r.defaultTier
	}

	ref, ok := r.byTier[tier]
	if !ok {
		return ModelRef{}, fmt.Errorf("%w: %q (configured: %s)",
			ErrTierNotConfigured, tier, joinTiers(r.configuredTiers()))
	}

	return ref, nil
}

// ResolveName validates and resolves a raw tier name straight from
// YAML — a checklist's `prompt_model:` or a prompt's `model:`. The
// empty string means the default tier.
func (r *Registry) ResolveName(raw string) (ModelRef, error) {
	if strings.TrimSpace(raw) == "" {
		return r.Resolve("")
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
