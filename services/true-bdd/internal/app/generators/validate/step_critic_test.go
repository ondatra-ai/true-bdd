package validate_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

type critiqueAI struct {
	model  provider.ModelRef
	role   provider.Role
	user   string
	answer string
}

func (a *critiqueAI) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	model provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions,
) (string, error) {
	a.model, a.role, a.user = model, role, user

	return a.answer, nil
}

func TestStepCriticRouting(t *testing.T) {
	t.Parallel()

	const high = "high"

	models, err := provider.NewRegistry(map[string]string{high: "claude:default", "quick": "codex:critic"},
		map[provider.Role]string{provider.RolePrompt: high, provider.RoleFix: high, provider.RoleApply: high})
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct{ tier, want string }{{"", "default"}, {"quick", "critic"}, {"missing", ""}} {
		t.Run("tier="+testCase.tier, func(t *testing.T) {
			t.Parallel()
			checkCriticRoute(t, models, testCase.tier, testCase.want)
		})
	}
}

func checkCriticRoute(t *testing.T, models *provider.Registry, tier, want string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "critic.tpl")

	err := disk.Write(path, []byte("{{.Definition}}\n{{.Context}}\n{{.Sources}}"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	client := &critiqueAI{answer: `{"answer":"fail","findings":[
 {"definition":"double","category":"phrase_mapping","finding":"missing operation"}]}`}
	critic := validate.NewStepCritic(client, models, path, path)
	data := validate.StepCritiqueData{Definition: "double", Context: "whole specification", Sources: "whole source"}

	findings, err := critic.Critique(context.Background(), data, tier, filepath.Join(dir, "turn"))
	if want == "" {
		if err == nil {
			t.Fatal("unknown tier accepted")
		}

		return
	}

	if err != nil || len(findings) != 1 {
		t.Fatalf("findings=%v error=%v", findings, err)
	}

	if client.model.Model != want || client.role != provider.RolePrompt ||
		!strings.Contains(client.user, "whole specification\nwhole source") {
		t.Fatalf("wrong request: %+v", client)
	}
}
