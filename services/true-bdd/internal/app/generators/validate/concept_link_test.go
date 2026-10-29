package validate_test

import (
	"context"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

type linkReply struct {
	response, schema string
	err              error
}

func (reply *linkReply) ExecutePromptWithSystem(_ context.Context, _ provider.Role, _, _ string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	reply.schema = schema

	return reply.response, reply.err
}

func TestLinkRejectsUnscoredConcept(t *testing.T) {
	const linkTestTier = "high"

	root := t.TempDir()
	t.Chdir(root)

	for path, body := range map[string]string{
		"true-bdd/true-bdd.yaml": "documents:\n  product: product.yaml\n",
		"product.yaml": "concepts:\n  when:\n" +
			"  - {concept: halve, means: Halve a number., terms: []}\n" +
			"  - {concept: double, means: Double a number., terms: []}\n",
	} {
		err := disk.Write(filepath.Join(root, path), []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{linkTestTier: "claude:test"}, map[provider.Role]string{
		provider.RolePrompt: linkTestTier, provider.RoleFix: linkTestTier, provider.RoleApply: linkTestTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	reply := &linkReply{response: `{"steps":{"1":{"scores":{"halve":10},"reason":"Halve.","other_block_concept":null}}}`}
	evaluator := validate.NewChecklistEvaluatorWithPaths(reply, cfg, models, "missing", "missing")

	result, err := evaluator.EvaluateOne(context.Background(), &template.ScenarioCheckData{
		ID: "halve", Service: "calc", Phrases: []catalogue.Step{{Block: catalogue.BlockWhen, Text: "halve 6"}},
	}, "halve", checklist.PromptWithContext{Index: 1, SectionID: "check", Prompt: checklist.Prompt{
		Question: "Link every step.", With: map[string]any{"concepts": "link"}, Docs: []string{"product"},
	}}, root)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != checklist.StatusFail || !strings.Contains(strings.Join(result.Context, "\n"), "double") {
		t.Fatalf("unscored concept must fail the cell: %+v", result)
	}

	if !strings.Contains(reply.schema, `"double"`) {
		t.Fatal("answer schema must require every concept")
	}

	rejectMissingLinkAnswer(t, evaluator, reply, root)
}

func rejectMissingLinkAnswer(t *testing.T, evaluator *validate.ChecklistEvaluator, reply *linkReply, root string) {
	t.Helper()

	reply.err = pkgerrors.ErrProviderProducedNoOutput("codex")

	missing, err := evaluator.EvaluateOne(context.Background(), &template.ScenarioCheckData{}, "missing",
		checklist.PromptWithContext{Prompt: checklist.Prompt{
			Question: "Link", With: map[string]any{"concepts": "link"}}}, root)
	if err != nil || missing.Status != checklist.StatusFail {
		t.Fatalf("missing answer: result=%+v, error=%v", missing, err)
	}
}
