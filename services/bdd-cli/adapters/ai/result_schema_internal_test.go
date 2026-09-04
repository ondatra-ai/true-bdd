package ai

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

// crush has no schema flag at all, so a turn routed to it can only be held
// to the delimited contract. This repo routes every apply turn there.
func TestSupportsResultSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cli  provider.CLI
		want bool
	}{
		{provider.CLIClaude, true},
		{provider.CLICodex, true},
		{provider.CLICrush, false},
	}

	for _, testCase := range tests {
		t.Run(string(testCase.cli), func(t *testing.T) {
			t.Parallel()

			if got := SupportsResultSchema(testCase.cli); got != testCase.want {
				t.Errorf("SupportsResultSchema(%s) = %v, want %v", testCase.cli, got, testCase.want)
			}
		})
	}
}

// A schema is only added when one was asked for: an unconstrained turn's
// argv must stay exactly what it was, or every cassette goes stale.
func TestOptionsAddSchemaOnlyWhenSet(t *testing.T) {
	t.Parallel()

	claudeProvider := &ClaudeProvider{}
	req := Request{SystemPrompt: "sys", Model: "claude-opus-5"}

	bare := claudeProvider.options(t.Context(), req).Args("body")

	req.ResultSchema = `{"type":"object"}`
	withSchema := claudeProvider.options(t.Context(), req).Args("body")

	if slices.Contains(bare, "--json-schema") {
		t.Errorf("unconstrained turn carries a schema: %q", bare)
	}

	if !slices.Contains(withSchema, "--json-schema") {
		t.Errorf("schema turn carries none: %q", withSchema)
	}
}

// Every engine turn is spawned isolated. This is the regression test for a
// run that inherited the operator's user settings and, through them, an
// advisor tool nobody configured — 121s and $2.39 of one 228s fixture.
func TestOptionsIsolateTheTurn(t *testing.T) {
	t.Parallel()

	args := (&ClaudeProvider{}).options(t.Context(), Request{Model: "claude-opus-5"}).Args("body")

	for _, want := range []string{"--setting-sources", "project", "--strict-mcp-config"} {
		if !slices.Contains(args, want) {
			t.Errorf("turn is not isolated, %q missing: %q", want, args)
		}
	}
}
