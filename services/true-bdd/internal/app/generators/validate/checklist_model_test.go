package validate_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/promptmodel"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestChecklistChecksTheHostSchemeBeforeRenderingOrAI(t *testing.T) {
	repository, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	t.Chdir(root)

	configuration := "templates:\n  schemes:\n    us_apply_checklist: host.scheme.yaml\n"

	err = disk.Write("true-bdd/true-bdd.yaml", []byte(configuration), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	source, err := disk.Read(filepath.Join(repository, "true-bdd/models/us_apply_checklist.schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// A host's narrowed field type must be enforced, even though rendering accepts the string.
	source = []byte(strings.Replace(string(source), "StoryID: *id001", "StoryID: {type: integer}", 1))

	err = disk.Write("host.scheme.yaml", source, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{linkTestTier: linkTestModel}, map[provider.Role]string{
		provider.RolePrompt: linkTestTier, provider.RoleFix: linkTestTier, provider.RoleApply: linkTestTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	reply := &linkReply{response: `{"answer":"pass","context":[]}`}
	evaluator := validate.NewChecklistEvaluatorWithPaths(
		reply, cfg, models, "missing-system.tpl", "missing-user.tpl", "us_apply_checklist",
	)
	subject := template.NewScenarioApplyData(
		"example", "story.yaml", "001-example", "example-001-example", "Example", nil, nil, nil, "tmp/registry.yaml",
	)

	_, err = evaluator.EvaluateOne(context.Background(), subject, "example-001-example", checklist.PromptWithContext{
		Index: 1, Prompt: checklist.Prompt{Question: "Compare this."},
	}, root)
	if !errors.Is(err, promptmodel.ErrModelData) || !strings.Contains(err.Error(), "StoryID") {
		t.Fatalf("want host field refusal before template loading, got %v", err)
	}

	if reply.user != "" {
		t.Fatal("invalid Model reached AI")
	}
}
