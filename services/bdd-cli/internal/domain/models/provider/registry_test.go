package provider_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
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

// validDefaults points every role at the same tier — the shape most
// tests need, where the defaults are incidental to what is asserted.
func validDefaults() map[provider.Role]string {
	defaults := make(map[provider.Role]string, len(provider.Roles()))
	for _, role := range provider.Roles() {
		defaults[role] = tierHigh
	}

	return defaults
}

func TestRegistryResolvesEachTier(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(validModels(), validDefaults())
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

// The empty tier is how "neither the prompt nor its checklist named one"
// reaches the registry, and each role falls back to its OWN
// engine.default_<role>_model — given a different tier per role here so a role-blind fallback cannot pass.
func TestRegistryEmptyTierUsesTheRoleDefault(t *testing.T) {
	t.Parallel()

	perRole := map[provider.Role]string{
		provider.RolePrompt: "xhigh",
		provider.RoleFix:    tierHigh,
		provider.RoleApply:  "coder",
	}

	registry, err := provider.NewRegistry(validModels(), perRole)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	wantModel := map[provider.Role]string{
		provider.RolePrompt: fableModel,
		provider.RoleFix:    opusModel,
		provider.RoleApply:  glmModel,
	}

	for _, role := range provider.Roles() {
		if got := string(registry.DefaultTier(role)); got != perRole[role] {
			t.Errorf("DefaultTier(%q) = %q, want %q", role, got, perRole[role])
		}

		byEmpty, resolveErr := registry.ResolveRole(role, "")
		if resolveErr != nil {
			t.Fatalf("ResolveRole(%q, \"\"): %v", role, resolveErr)
		}

		byBlank, resolveErr := registry.ResolveRole(role, "   ")
		if resolveErr != nil {
			t.Fatalf("ResolveRole(%q, blank): %v", role, resolveErr)
		}

		if byEmpty != byBlank || byEmpty.Model != wantModel[role] {
			t.Errorf("role %q default resolved to %v / %v, want model %q",
				role, byEmpty, byBlank, wantModel[role])
		}
	}
}

// A named tier still wins over the role's default — the per-role
// fallback must only apply when the YAML said nothing.
func TestRegistryNamedTierOverridesRoleDefault(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(validModels(), validDefaults())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	ref, err := registry.ResolveRole(provider.RoleApply, "xhigh")
	if err != nil {
		t.Fatalf("ResolveRole(apply, xhigh): %v", err)
	}

	if ref.Model != fableModel {
		t.Errorf("ResolveRole(apply, xhigh) = %v, want the xhigh model", ref)
	}
}

func TestRegistryResolveRoleRejectsUnknownTier(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(validModels(), validDefaults())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	_, err = registry.ResolveRole(provider.RolePrompt, "turbo")
	if !errors.Is(err, provider.ErrUnknownTier) {
		t.Fatalf("ResolveRole(prompt, \"turbo\") error = %v, want ErrUnknownTier", err)
	}
}

// A tier named by a checklist but absent from engine.models must fail
// rather than silently fall back to the default model.
func TestRegistryResolveUnconfiguredTier(t *testing.T) {
	t.Parallel()

	registry, err := provider.NewRegistry(map[string]string{tierHigh: opusRef}, validDefaults())
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

	// withRole overrides one role's default, leaving the rest valid, so
	// a failing case can only be blamed on the role it names.
	withRole := func(role provider.Role, tier string) map[provider.Role]string {
		defaults := validDefaults()
		defaults[role] = tier

		return defaults
	}

	tests := []struct {
		name     string
		models   map[string]string
		defaults map[provider.Role]string
		wantErr  error
		wantKey  string // substring the message must name, "" to skip
	}{
		{
			name:     "no models at all",
			models:   nil,
			defaults: validDefaults(),
			wantErr:  provider.ErrNoModelsConfigured,
		},
		{
			name:     "tier name typo",
			models:   map[string]string{"hihg": opusRef},
			defaults: validDefaults(),
			wantErr:  provider.ErrUnknownTier,
		},
		{
			name:     "model reference missing the cli prefix",
			models:   map[string]string{tierHigh: opusModel},
			defaults: validDefaults(),
			wantErr:  provider.ErrModelRefNoSeparator,
		},
		{
			name:     "unknown cli",
			models:   map[string]string{tierHigh: "aider:whatever"},
			defaults: validDefaults(),
			wantErr:  provider.ErrModelRefUnknownCLI,
		},
		{
			name:     "prompt default names an unknown tier",
			models:   validModels(),
			defaults: withRole(provider.RolePrompt, "medium"),
			wantErr:  provider.ErrUnknownTier,
			wantKey:  "engine.default_prompt_model",
		},
		{
			name:     "apply default is missing entirely",
			models:   validModels(),
			defaults: withRole(provider.RoleApply, ""),
			wantErr:  provider.ErrUnknownTier,
			wantKey:  "engine.default_apply_model",
		},
		{
			name:     "fix default names a tier that is not configured",
			models:   map[string]string{tierHigh: opusRef},
			defaults: withRole(provider.RoleFix, "coder"),
			wantErr:  provider.ErrDefaultTierMissing,
			wantKey:  "engine.default_fix_model",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := provider.NewRegistry(testCase.models, testCase.defaults)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewRegistry error = %v, want %v", err, testCase.wantErr)
			}

			// The message must name the exact key to fix — that is the
			// difference between a one-line fix and a hunt.
			if testCase.wantKey != "" && !strings.Contains(err.Error(), testCase.wantKey) {
				t.Errorf("NewRegistry error = %q, want it to name %q", err, testCase.wantKey)
			}
		})
	}
}
