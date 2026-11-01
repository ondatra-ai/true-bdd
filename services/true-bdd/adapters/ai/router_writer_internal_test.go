package ai

import (
	"context"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
)

type captureTurn struct{ request Request }

func (*captureTurn) Name() string { return "codex" }
func (p *captureTurn) Execute(_ context.Context, request Request) (string, error) {
	p.request = request

	return "answer", nil
}

func TestRouterConfinesEveryCodexApply(t *testing.T) {
	for _, role := range []provider.Role{provider.RoleApply, provider.RoleFix, provider.RolePrompt} {
		for _, requested := range []bool{false, true} {
			for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
				target := &captureTurn{}
				router := &Router{providers: map[provider.CLI]Provider{cli: target}}

				_, err := router.ExecutePromptWithSystem(t.Context(), role, "system", "user",
					provider.ModelRef{CLI: cli, Model: "test"}, ExecutionMode{}, "",
					provider.TurnOptions{CodexWriter: requested})
				if err != nil {
					t.Fatal(err)
				}

				assertCodexConfinement(t, target.request, role, cli, requested)
			}
		}
	}
}

func TestEveryRouterTurnIsUnbounded(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		for _, role := range provider.Roles() {
			for _, requested := range []bool{false, true} {
				target := &deadlineTurn{}
				router := &Router{providers: map[provider.CLI]Provider{cli: target}}

				_, err := router.ExecutePromptWithSystem(t.Context(), role, "s", "u", provider.ModelRef{CLI: cli,
					Model: "test"}, ExecutionMode{}, "", provider.TurnOptions{ClaudeWriter: requested, CodexWriter: requested})
				if err != nil {
					t.Fatal(err)
				}

				want := requested && cli == provider.CLIClaude && role == provider.RoleApply
				target.assertWriterDeadline(t, want)
			}
		}
	}
}

type deadlineTurn struct {
	request Request
	bounded bool
}

func (*deadlineTurn) Name() string { return "claude" }
func (d *deadlineTurn) Execute(ctx context.Context, req Request) (string, error) {
	d.request = req
	_, bounded := ctx.Deadline()
	d.bounded = bounded

	return "completed", nil
}

func (d *deadlineTurn) assertWriterDeadline(t *testing.T, want bool) {
	t.Helper()

	if d.request.ClaudeWriter != want || d.bounded {
		t.Fatalf("bounded=%v writer=%v want=%v", d.bounded, d.request.ClaudeWriter, want)
	}
}

func assertCodexConfinement(t *testing.T, req Request, role provider.Role, cli provider.CLI, requested bool) {
	t.Helper()

	want := role == provider.RoleApply && cli == provider.CLICodex
	if req.CodexApply != want {
		t.Fatalf("role=%s cli=%s requested=%v: confinement=%v", role, cli, requested, req.CodexApply)
	}
}

func TestConcurrentValidationArtifactLabelsIgnoreDispatchOrder(t *testing.T) {
	t.Parallel()

	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		target := &captureTurn{}

		router := &Router{providers: map[provider.CLI]Provider{cli: target}}
		for _, key := range []string{"validation-beta", "validation-alpha", "validation-beta"} {
			_, err := router.ExecutePromptWithSystem(t.Context(), provider.RolePrompt, "s", "u",
				provider.ModelRef{CLI: cli, Model: "concurrent-test"}, ExecutionMode{}, "",
				provider.TurnOptions{ArtifactLabel: key})
			if err != nil {
				t.Fatal(err)
			}

			if target.request.Label != key {
				t.Fatalf("got label %s, want %s", target.request.Label, key)
			}
		}
	}
}
