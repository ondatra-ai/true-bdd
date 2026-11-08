package ai_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
)

func TestRouterPromptByteLimit(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		t.Run(string(cli), func(t *testing.T) {
			root := promptLimitCLI(t, cli)

			for _, limit := range []int{150000, 500} {
				for _, role := range provider.Roles() {
					for _, delta := range []int{-1, 0, 1} {
						checkPromptLimit(t, root, cli, role, limit, delta)
					}
				}
			}
		})
	}
}

func promptLimitCLI(t *testing.T, cli provider.CLI) string {
	t.Helper()
	root := t.TempDir()

	launcher := "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"result\",\"result\":\"ok\"}'\n"
	if cli == provider.CLICodex {
		launcher = "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n" +
			"if [ \"$1\" = -o ]; then shift; printf ok > \"$1\"; fi\nshift\ndone\n"
	}

	err := os.WriteFile(filepath.Join(root, string(cli)), []byte(launcher), 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll(filepath.Join(root, "tmp/run"), 0750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Mkdir(filepath.Join(root, "auth"), 0700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(root, "auth/auth.json"), []byte(`{"fixture":"dummy"}`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "auth"))

	return root
}

func checkPromptLimit(t *testing.T, root string, cli provider.CLI, role provider.Role, limit, delta int) {
	t.Helper()

	overhead := 0
	if cli == provider.CLICodex {
		overhead = len("### SYSTEM INSTRUCTIONS (authoritative — follow exactly)\n\n\n\n### TASK\n\n")
	}

	router := ai.NewRouter(root, filepath.Join(root, "tmp/run"), limit)
	if limit == 150000 {
		router = ai.NewRouter(root, filepath.Join(root, "tmp/run"))
	}

	_, err := router.ExecutePromptWithSystem(t.Context(), role, "é", strings.Repeat("x", limit-2-overhead+delta),
		provider.ModelRef{CLI: cli, Model: "test"}, provider.ExecutionMode{}, "")
	if delta <= 0 {
		if err != nil {
			t.Fatalf("under limit: %v", err)
		}

		return
	}

	var refused *ai.PromptLimitError
	if !errors.As(err, &refused) {
		t.Fatalf("want prompt refusal, got %v", err)
	}

	if refused.Size != limit+1 || refused.Limit != limit {
		t.Fatalf("wrong size or limit: %v", err)
	}

	for _, identity := range []string{string(cli), string(role)} {
		if !strings.Contains(err.Error(), identity) {
			t.Fatalf("missing turn identity: %v", err)
		}
	}
}
