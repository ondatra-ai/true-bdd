package ai_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestRouterCancellationStopsEveryTurnProcessGroup(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		for _, role := range provider.Roles() {
			for _, writer := range []bool{false, true} {
				if writer && role != provider.RoleApply {
					continue
				}

				name := string(cli) + "/" + string(role) + "/ordinary"
				if writer {
					name = string(cli) + "/writer"
				}

				t.Run(name, func(t *testing.T) { cancelRouterTurn(t, cli, role, writer) })
			}
		}
	}
}

func cancelRouterTurn(t *testing.T, cli provider.CLI, role provider.Role, writer bool) {
	t.Helper()

	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	// The ready descendant execs sleep, avoiding a fork racing with the cancellation signal.
	putCodexFile(t, filepath.Join(bin, string(cli)), `#!/bin/sh
/bin/sh -c 'printf ready > started; exec /bin/sleep 30'
exit 0
`)
	putCodexFile(t, filepath.Join(root, "auth/auth.json"), "{}")
	t.Setenv("CODEX_HOME", filepath.Join(root, "auth"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "claude-state"))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan struct{})

	defer func() {
		cancel()
		<-finished
	}()

	done := make(chan error, 1)

	go func() {
		defer close(finished)

		_, err := ai.NewRouter(root, filepath.Join(root, "tmp")).ExecutePromptWithSystem(ctx, role,
			"system", "user", provider.ModelRef{CLI: cli, Model: "test"}, ai.ExecutionMode{}, "",
			provider.TurnOptions{ClaudeWriter: writer, CodexWriter: writer})
		done <- err
	}()

	waitForTurnStart(t, root, done)

	start := time.Now()

	cancel()

	err := <-done

	var (
		claudeFailure *pkgerrors.ClaudeTurnError
		codexFailure  *pkgerrors.CodexTurnError
	)
	if !errors.Is(err, context.Canceled) || errors.As(err, &claudeFailure) || errors.As(err, &codexFailure) {
		t.Fatalf("cancellation must stay fatal and preserve its cause: %v", err)
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("descendant retained the turn's pipes after cancellation: %s", elapsed)
	}
}

func waitForTurnStart(t *testing.T, root string, done <-chan error) {
	t.Helper()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		_, err := os.Stat(filepath.Join(root, "started"))
		if err == nil {
			return
		}

		select {
		case err := <-done:
			t.Fatalf("turn ended before starting: %v", err)
		case <-deadline.C:
			t.Fatal("turn did not start")
		case <-tick.C:
		}
	}
}
