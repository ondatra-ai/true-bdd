package ai

import (
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
func TestBuildClientOptionsAddsSchemaOnlyWhenSet(t *testing.T) {
	t.Parallel()

	claude := &ClaudeProvider{}
	mode := ExecutionMode{}

	bare := claude.buildClientOptions("sys", "claude-opus-5", mode, "")
	withSchema := claude.buildClientOptions("sys", "claude-opus-5", mode, `{"type":"object"}`)

	if len(withSchema) != len(bare)+1 {
		t.Fatalf("options = %d with schema vs %d without, want exactly one more",
			len(withSchema), len(bare))
	}
}
