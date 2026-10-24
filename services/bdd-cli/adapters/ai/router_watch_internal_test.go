package ai

import (
	"context"
	"testing"
	"time"

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
					provider.TurnOptions{WatchWriterResponse: requested})
				if err != nil {
					t.Fatal(err)
				}

				want := requested && role == provider.RoleApply && cli == provider.CLICrush
				if target.request.WatchWriterResponse != want {
					t.Fatalf("role=%s cli=%s requested=%v: watch=%v", role, cli, requested, target.request.WatchWriterResponse)
				}
			}
		}
	}
}

func TestOnlyExplicitClaudeWriterIsUnbounded(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICrush, provider.CLICodex} {
		for _, role := range provider.Roles() {
			for _, requested := range []bool{false, true} {
				target := &deadlineTurn{}
				router := &Router{providers: map[provider.CLI]Provider{cli: target}}

				_, err := router.ExecutePromptWithSystem(t.Context(), role, "s", "u", provider.ModelRef{CLI: cli,
					Model: "test"}, ExecutionMode{}, "", provider.TurnOptions{ClaudeWriter: requested})
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
	remaining time.Duration
	request   Request
	bounded   bool
}

func (*deadlineTurn) Name() string { return "claude" }
func (d *deadlineTurn) Execute(ctx context.Context, req Request) (string, error) {
	d.request = req
	deadline, bounded := ctx.Deadline()
	d.bounded = bounded
	d.remaining = time.Until(deadline)

	return "completed", nil
}

func (d *deadlineTurn) assertWriterDeadline(t *testing.T, want bool) {
	t.Helper()

	if d.request.ClaudeWriter != want || d.bounded == want {
		t.Fatalf("bounded=%v writer=%v want=%v", d.bounded, d.request.ClaudeWriter, want)
	}

	if d.bounded && (d.remaining > 20*time.Minute || d.remaining < 20*time.Minute-time.Second) {
		t.Fatalf("deadline=%s, want 20m", d.remaining)
	}
}
