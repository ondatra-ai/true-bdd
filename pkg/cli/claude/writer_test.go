package claude_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

func TestWriterArgumentsAndEnvironment(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SAFE_MODE", "1")

	opts := claude.Options{Minimal: true, Writer: &claude.WriterOptions{Settings: "/host/tmp/settings.json",
		ReadDirs: []string{"/toolchain", "/own-spills"}, SessionID: "test-session"}}

	args := opts.Args("body")
	for _, flag := range []string{"--restricted", "--settings", "--disable-slash-commands", "--tools", "--session-id"} {
		if !slices.Contains(args, flag) {
			t.Errorf("missing %s: %v", flag, args)
		}
	}

	if slices.Contains(args, "--safe-mode") || slices.Contains(args, "Bash(*)") {
		t.Fatalf("writer args: %v", args)
	}

	result, err := shell.Run(t.Context(), []string{"/bin/bash", "-eu", "-c",
		`printf "%s %s %s" "${CLAUDE_CODE_SAFE_MODE+x}" "$BASH_DEFAULT_TIMEOUT_MS" "$BASH_MAX_TIMEOUT_MS"`},
		shell.Options{Env: opts.Env()})
	if err != nil {
		t.Fatal(err)
	}

	env := result.Stdout
	if env != " 600000 600000" {
		t.Fatalf("writer environment: %s", env)
	}

	ordinary := claude.Options{Minimal: true}.Args("body")
	if !slices.Contains(ordinary, "--safe-mode") || slices.Contains(ordinary, "--restricted") {
		t.Fatal(ordinary)
	}
}

func TestStreamStartedOutcomesKeepUsage(t *testing.T) {
	for _, testCase := range []struct {
		name, body string
		failed     bool
	}{
		{"nonzero",
			`printf '%s\n' '{"type":"result","result":"changed","total_cost_usd":1.25,` +
				`"usage":{"output_tokens":7}}'; exit 7`, true},
		{"error result",
			`printf '%s\n' '{"type":"result","is_error":true,"subtype":"error_during_execution",` +
				`"errors":["rate_limit"],"api_error_status":429,"total_cost_usd":1.25}'`, true},
		{"empty", `exit 0`, true},
		{"success", `printf '%s\n' '{"type":"result","result":"changed","total_cost_usd":1.25}'`, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()

			err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+testCase.body+"\n"), 0755)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("PATH", dir)
			answer, err := claude.RunStreamContext(t.Context(), "prompt", claude.Options{}, func(claude.Event) {})

			var started *claude.StartedTurnError
			if errors.As(err, &started) != testCase.failed {
				t.Fatalf("error=%v; want started=%v", err, testCase.failed)
			}

			if testCase.name != "empty" && answer.CostUSD != 1.25 {
				t.Fatalf("lost cost: %+v", answer)
			}
		})
	}

	t.Setenv("PATH", t.TempDir())
	_, err := claude.RunStreamContext(t.Context(), "prompt", claude.Options{}, func(claude.Event) {})

	var started *claude.StartedTurnError
	if err == nil || errors.As(err, &started) {
		t.Fatalf("start failure = %v", err)
	}
}

func TestUnboundedWriterCancellationStopsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	// The writer and its child both retain the output pipes until cancelled.
	script := "#!/bin/sh\nsleep 30\n"

	err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()

	_, err = claude.RunStreamContext(ctx, "prompt", claude.Options{Writer: &claude.WriterOptions{}}, func(claude.Event) {})
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("cancel took %s: %v", time.Since(start), err)
	}
}

func TestAssistantErrorWithoutEnvelopeKeepsDiagnostic(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
printf '%s\n' '{"type":"assistant","error":"rate_limit","message":{"content":[]}}'
`

	err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	_, err = claude.RunStreamContext(t.Context(), "prompt", claude.Options{}, func(claude.Event) {})

	var started *claude.StartedTurnError
	if !errors.As(err, &started) || started.Outcome != claude.TurnErrorResult {
		t.Fatalf("error=%v", err)
	}

	if !strings.Contains(err.Error(), "rate_limit") {
		t.Fatalf("lost diagnostic: %v", err)
	}
}
