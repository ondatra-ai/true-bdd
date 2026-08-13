package provider_test

import (
	"errors"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

const (
	fableModel = "claude-fable-5"
	opusModel  = "claude-opus-4-8"
	glmModel   = "zhipu-coding/glm-5.2"
)

func TestParseModelRefValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		wantCLI   provider.CLI
		wantModel string
	}{
		{
			name:      "claude with hyphenated id",
			raw:       "claude:" + fableModel,
			wantCLI:   provider.CLIClaude,
			wantModel: fableModel,
		},
		{
			name:      "crush with provider-qualified id",
			raw:       "crush:" + glmModel,
			wantCLI:   provider.CLICrush,
			wantModel: glmModel,
		},
		{
			// Only the FIRST colon splits, so a model id that itself
			// contains one survives intact.
			name:      "codex with colon inside the model id",
			raw:       "codex:openai:gpt-5.6-sol",
			wantCLI:   provider.CLICodex,
			wantModel: "openai:gpt-5.6-sol",
		},
		{
			name:      "surrounding whitespace is trimmed",
			raw:       "  claude : claude-opus-4-8  ",
			wantCLI:   provider.CLIClaude,
			wantModel: opusModel,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ref, err := provider.ParseModelRef(testCase.raw)
			if err != nil {
				t.Fatalf("ParseModelRef(%q): unexpected error: %v", testCase.raw, err)
			}

			if ref.CLI != testCase.wantCLI {
				t.Errorf("cli = %q, want %q", ref.CLI, testCase.wantCLI)
			}

			if ref.Model != testCase.wantModel {
				t.Errorf("model = %q, want %q", ref.Model, testCase.wantModel)
			}
		})
	}
}

func TestParseModelRefRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{
			// The whole point of requiring the cli prefix: a bare model
			// id must not be silently routed to some default CLI.
			name:    "bare model id",
			raw:     fableModel,
			wantErr: provider.ErrModelRefNoSeparator,
		},
		{name: "empty cli", raw: ":" + fableModel, wantErr: provider.ErrModelRefEmptyCLI},
		{name: "empty model", raw: "claude:", wantErr: provider.ErrModelRefEmptyModel},
		{name: "unknown cli", raw: "aider:some-model", wantErr: provider.ErrModelRefUnknownCLI},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := provider.ParseModelRef(testCase.raw)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("ParseModelRef(%q) error = %v, want %v", testCase.raw, err, testCase.wantErr)
			}
		})
	}
}

func TestModelRefString(t *testing.T) {
	t.Parallel()

	ref := provider.ModelRef{CLI: provider.CLICrush, Model: glmModel}

	const want = "crush:" + glmModel
	if ref.String() != want {
		t.Errorf("String() = %q, want %q", ref.String(), want)
	}

	if ref.IsZero() {
		t.Error("IsZero() = true for a populated ref")
	}

	if !(provider.ModelRef{}).IsZero() {
		t.Error("IsZero() = false for the zero ref")
	}
}

func TestParseTier(t *testing.T) {
	t.Parallel()

	for _, tier := range provider.Tiers() {
		parsed, err := provider.ParseTier(string(tier))
		if err != nil {
			t.Fatalf("ParseTier(%q): unexpected error: %v", tier, err)
		}

		if parsed != tier {
			t.Errorf("ParseTier(%q) = %q", tier, parsed)
		}
	}

	_, err := provider.ParseTier("medium")
	if !errors.Is(err, provider.ErrUnknownTier) {
		t.Fatalf("ParseTier(\"medium\") error = %v, want ErrUnknownTier", err)
	}
}
