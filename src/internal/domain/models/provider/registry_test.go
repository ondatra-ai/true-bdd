package provider_test

import (
	"errors"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/domain/models/provider"
)

const (
	fableRef = "claude:" + fableModel
	opusRef  = "claude:" + opusModel
	glmRef   = "crush:" + glmModel

	tierHigh = "high"
)

func validModels() map[string]string {
	return map[string]string{
		"xhigh":  fableRef,
		tierHigh: opusRef,
		"coder":  glmRef,
	}
}

func TestRegistryResolvesEachTier(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(validModels(), tierHigh)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	coder, err := registry.Resolve(provider.TierCoder)
	if err != nil {
		t.Fatalf("Resolve(coder): %v", err)
	}

	if coder.CLI != provider.CLICrush || coder.Model != glmModel {
		t.Errorf("Resolve(coder) = %v", coder)
	}
}

// The empty tier is how "the checklist said nothing" reaches the
// registry — it must land on engine.default_model, not on an error.
func TestRegistryEmptyTierUsesDefault(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(validModels(), "xhigh")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	if registry.DefaultTier() != provider.TierXHigh {
		t.Fatalf("DefaultTier() = %q", registry.DefaultTier())
	}

	byEmptyTier, err := registry.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\"): %v", err)
	}

	byEmptyName, err := registry.ResolveName("   ")
	if err != nil {
		t.Fatalf("ResolveName(blank): %v", err)
	}

	if byEmptyTier != byEmptyName || byEmptyTier.Model != fableModel {
		t.Errorf("empty tier resolved to %v / %v, want the xhigh model", byEmptyTier, byEmptyName)
	}
}

func TestRegistryResolveNameRejectsUnknownTier(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(validModels(), tierHigh)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	_, err = registry.ResolveName("turbo")
	if !errors.Is(err, provider.ErrUnknownTier) {
		t.Fatalf("ResolveName(\"turbo\") error = %v, want ErrUnknownTier", err)
	}
}

// A tier named by a checklist but absent from engine.models must fail
// rather than silently fall back to the default model.
func TestRegistryResolveUnconfiguredTier(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(map[string]string{tierHigh: opusRef}, "high")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	_, err = registry.Resolve(provider.TierCoder)
	if !errors.Is(err, provider.ErrTierNotConfigured) {
		t.Fatalf("Resolve(coder) error = %v, want ErrTierNotConfigured", err)
	}
}

func TestNewRegistryRejectsBadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		models       map[string]string
		defaultModel string
		wantErr      error
	}{
		{
			name:         "no models at all",
			models:       nil,
			defaultModel: tierHigh,
			wantErr:      provider.ErrNoModelsConfigured,
		},
		{
			name:         "tier name typo",
			models:       map[string]string{"hihg": opusRef},
			defaultModel: tierHigh,
			wantErr:      provider.ErrUnknownTier,
		},
		{
			name:         "model reference missing the cli prefix",
			models:       map[string]string{tierHigh: opusModel},
			defaultModel: tierHigh,
			wantErr:      provider.ErrModelRefNoSeparator,
		},
		{
			name:         "unknown cli",
			models:       map[string]string{tierHigh: "aider:whatever"},
			defaultModel: tierHigh,
			wantErr:      provider.ErrModelRefUnknownCLI,
		},
		{
			name:         "default names an unknown tier",
			models:       validModels(),
			defaultModel: "medium",
			wantErr:      provider.ErrUnknownTier,
		},
		{
			name:         "default names a tier that is not configured",
			models:       map[string]string{tierHigh: opusRef},
			defaultModel: "coder",
			wantErr:      provider.ErrDefaultTierMissing,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := provider.NewRegistry(testCase.models, testCase.defaultModel)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewRegistry error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}
