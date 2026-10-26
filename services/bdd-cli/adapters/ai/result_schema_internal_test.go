package ai

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

func TestSupportsResultSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cli  provider.CLI
		want bool
	}{
		{provider.CLIClaude, true},
		{provider.CLICodex, true},
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

	bare := claudeProvider.options(req).Args("body")

	req.ResultSchema = `{"type":"object"}`
	withSchema := claudeProvider.options(req).Args("body")

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

	args := (&ClaudeProvider{}).options(Request{Model: "claude-opus-5"}).Args("body")

	want := []string{"--setting-sources", "", "--strict-mcp-config", "--safe-mode"}
	if !slices.Equal(args[:len(want)], want) {
		t.Errorf("turn is not minimal, got argv %q, want prefix %q", args, want)
	}
}
