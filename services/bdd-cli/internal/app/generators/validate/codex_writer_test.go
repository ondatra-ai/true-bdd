package validate_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

const writerTier = "coder"

func TestBuildCodeApplierRequestsCodexConfinementAndNativePolling(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	for path, body := range map[string]string{
		"true-bdd/true-bdd.yaml": `engine:
  models:
    coder: codex:gpt-6.1-sol
  default_apply_model: coder
  default_prompt_model: high
  default_fix_model: high
`,
		"system.tpl": "Apply.",
		"user.tpl":   "{{if .CodexWriter}}poll process{{end}} {{.TestCommand.Command}} {{.ResultPath}}",
	} {
		err := disk.Write(path, []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{writerTier: "codex:gpt-6.1-sol"},
		map[provider.Role]string{provider.RoleApply: writerTier, provider.RoleFix: writerTier,
			provider.RolePrompt: writerTier,
		})
	if err != nil {
		t.Fatal(err)
	}

	client := &writerCapture{}
	applier := validate.NewFixApplierWithPaths(client, cfg, models, "system.tpl", "user.tpl")
	applier.UseWriteRoots([]string{"services/demo"})
	applier.UseTestCommand(&provider.TestCommand{Command: "go test ./tests/suite", Selector: "-run", Dir: root})

	_, _ = applier.Apply(t.Context(), validate.ApplyParams{TmpDir: filepath.Join(root, "tmp"), CodexWriter: true})
	if !client.confined || !strings.Contains(client.user, "poll process go test ./tests/suite") {
		t.Fatalf("confined=%v prompt=%q", client.confined, client.user)
	}
}

type writerCapture struct {
	confined bool
	user     string
}

func (c *writerCapture) ExecutePromptWithSystem(_ context.Context, _ provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, options ...provider.TurnOptions) (string, error) {
	c.user = user
	c.confined = len(options) > 0 && options[0].CodexWriter

	return "no confirmation", nil
}
