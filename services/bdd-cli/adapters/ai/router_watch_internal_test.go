package ai

import (
	"context"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
)

type captureTurn struct{ request Request }

func (*captureTurn) Name() string { return "crush" }
func (p *captureTurn) Execute(_ context.Context, request Request) (string, error) {
	p.request = request

	return "answer", nil
}

func TestRouterWatchesOnlyAnExplicitCrushApply(t *testing.T) {
	for _, role := range []provider.Role{provider.RoleApply, provider.RoleFix, provider.RolePrompt} {
		for _, requested := range []bool{false, true} {
			for _, cli := range []provider.CLI{provider.CLICrush, provider.CLIClaude, provider.CLICodex} {
				target := &captureTurn{}
				router := &Router{providers: map[provider.CLI]Provider{cli: target}}

				_, err := router.ExecutePromptWithSystem(t.Context(), role, "system", "user",
					provider.ModelRef{CLI: cli, Model: "test"}, ExecutionMode{}, "",
					provider.TurnOptions{WatchWriterResponse: requested, CodexWriter: requested})
				if err != nil {
					t.Fatal(err)
				}

				want := requested && role == provider.RoleApply && cli == provider.CLICrush
				if target.request.WatchWriterResponse != want {
					t.Fatalf("role=%s cli=%s requested=%v: watch=%v", role, cli, requested, target.request.WatchWriterResponse)
				}

				assertCodexConfinement(t, target.request, role, cli, requested)
			}
		}
	}
}

func assertCodexConfinement(t *testing.T, req Request, role provider.Role, cli provider.CLI, requested bool) {
	t.Helper()

	want := requested && role == provider.RoleApply && cli == provider.CLICodex
	if req.CodexWriter != want {
		t.Fatalf("role=%s cli=%s requested=%v: confinement=%v", role, cli, requested, req.CodexWriter)
	}
}
