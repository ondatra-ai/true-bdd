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
	system, user     string
	err              error
}

func (reply *linkReply) ExecutePromptWithSystem(_ context.Context, _ provider.Role, system, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	reply.schema = schema
	reply.system, reply.user = system, user

	return reply.response, reply.err
}

const linkTestTier = "high"
const linkTestModel = "claude:test"
const linkJudgement = "link"
const conceptsParameter = "concepts"
const linkTestConcept = "square"

func TestConceptLinkThresholdChangesSameAnswerVerdict(t *testing.T) {
	evaluator, reply, repository := conceptRequestEvaluator(t)
	reply.response = `{"steps":{"1":{"scores":{"square":8,"halve":2},` +
		`"reason":"Related but partly outside square's meaning.","other_block_concept":null}}}`

	for _, testCase := range []struct {
		name      string
		threshold int
		status    checklist.Status
		links     int
	}{
		{"eight", 8, checklist.StatusPass, 1},
		{"nine", 9, checklist.StatusFail, 0},
		{"omitted", 0, checklist.StatusFail, 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			prompt := shippedConceptPrompt(t, repository, linkJudgement)
			delete(prompt.Prompt.With, "threshold")

			if testCase.threshold != 0 {
				prompt.Prompt.With["threshold"] = testCase.threshold
			}

			item := &template.ScenarioCheckData{ID: linkTestConcept, Phrases: []catalogue.Step{
				{Block: catalogue.BlockWhen, Text: "square six approximately"},
			}}

			result, err := evaluator.EvaluateOne(t.Context(), item, item.ID, prompt, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			if result.Status != testCase.status {
				t.Fatalf("same highest score eight: got %s, want %s; %v", result.Status, testCase.status, result.Context)
			}

			if item.ConceptLinkPassed != (testCase.status == checklist.StatusPass) || len(item.ConceptLinks) != testCase.links {
				t.Fatalf("score eight: want %d links; got %+v", testCase.links, item)
			}
		})
	}
}

func TestLinkRejectsUnscoredConcept(t *testing.T) {
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

	models, err := provider.NewRegistry(map[string]string{linkTestTier: linkTestModel}, map[provider.Role]string{
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
		Question: "Link every step.", With: map[string]any{conceptsParameter: linkJudgement}, Docs: []string{"product"},
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
			Question: "Link", With: map[string]any{conceptsParameter: linkJudgement}}}, root)
	if err != nil || missing.Status != checklist.StatusFail {
		t.Fatalf("missing answer: result=%+v, error=%v", missing, err)
	}
}
