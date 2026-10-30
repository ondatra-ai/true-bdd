package validate_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func conceptRequestEvaluator(t *testing.T) (*validate.ChecklistEvaluator, *linkReply, string) {
	t.Helper()

	repository, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	t.Chdir(root)

	for name, body := range map[string]string{
		filepath.Join("true-bdd", "true-bdd.yaml"): "documents:\n  product: product.yaml\n" +
			"  architecture_yaml: private-architecture.yaml\n",
		"product.yaml": "roles: [{name: Owner}]\nglossary: []\nconcepts:\n  when:\n" +
			"  - {concept: square, means: Square the entered number., terms: []}\n" +
			"  - {concept: halve, means: Halve the entered number., terms: []}\n" +
			"  then:\n  - {concept: result, means: Compare the displayed number., terms: []}\n",
		"private-architecture.yaml": "architecture:\n  services:\n  - name: calc\n    public_interface:\n" +
			"      kind: ui\n      elements: [{page: Calc, role: button, name: Square}]\n" +
			"  concepts: [{concept: secret-binding-marker}]\n",
	} {
		err = disk.Write(filepath.Join(root, name), []byte(body), disk.Shared)
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

	reply := &linkReply{response: `{"answer":"pass","context":["convertible"]}`}
	evaluator := validate.NewChecklistEvaluatorWithPaths(reply, cfg, models,
		filepath.Join(repository, "templates/us-checklist.system.prompt.tpl"),
		filepath.Join(repository, "templates/us-checklist.prompt.tpl"))

	return evaluator, reply, repository
}

func shippedConceptPrompt(t *testing.T, repository, kind string) checklist.PromptWithContext {
	t.Helper()

	body, err := disk.Read(filepath.Join(repository, "true-bdd/checklists/us-refine.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var document checklist.Checklist

	err = yaml.Unmarshal(body, &document)
	if err != nil {
		t.Fatal(err)
	}

	for _, section := range document.Sections {
		for _, prompt := range section.ValidationPrompts {
			if prompt.With[conceptsParameter] == kind {
				return checklist.PromptWithContext{Index: 1, SectionID: section.ID, Prompt: prompt}
			}
		}
	}

	t.Fatalf("no shipped %s Prompt", kind)

	return checklist.PromptWithContext{}
}

func TestConceptLinkRequestHasClosedMeaningsAndRequiredBlockScores(t *testing.T) {
	evaluator, reply, repository := conceptRequestEvaluator(t)
	item := &template.ScenarioCheckData{ID: linkTestConcept, Service: linkTestService, Phrases: []catalogue.Step{
		{Block: catalogue.BlockWhen, Text: linkTestStepText},
		{Block: catalogue.BlockThen, Text: "36 is displayed"},
		{Block: catalogue.BlockThen, Text: "judge: the explanation is clear"},
	}}

	_, err := evaluator.EvaluateOne(context.Background(), item, item.ID,
		shippedConceptPrompt(t, repository, linkJudgement), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, required := range []string{"CLOSED", "at most 8", "at most 4"} {
		if !strings.Contains(reply.system, required) {
			t.Errorf("link scoring instructions omit %q", required)
		}
	}

	if strings.Contains(strings.ToLower(reply.system+reply.user), "architecture") ||
		!strings.Contains(reply.user, "Square the entered number.") {
		t.Fatalf("link must embed meanings without architecture: %s", reply.user)
	}

	assertRequiredBlockScores(t, reply.schema)
}

func TestConceptLinkFailsAStepOfABlockWithoutConceptsUnderAValidSchema(t *testing.T) {
	evaluator, reply, repository := conceptRequestEvaluator(t)
	reply.response = `{"steps":{"1":{"scores":{},"reason":"No given concept is declared.","other_block_concept":null}}}`
	item := &template.ScenarioCheckData{ID: linkTestConcept, Service: linkTestService, Phrases: []catalogue.Step{
		{Block: catalogue.BlockGiven, Text: "six is entered"},
	}}

	result, err := evaluator.EvaluateOne(context.Background(), item, item.ID,
		shippedConceptPrompt(t, repository, linkJudgement), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var contract struct {
		Properties struct {
			Steps struct {
				Properties map[string]struct {
					Properties struct {
						Scores struct {
							Required *[]string `json:"required"`
						} `json:"scores"`
					} `json:"properties"`
				} `json:"properties"`
			} `json:"steps"`
		} `json:"properties"`
	}

	err = json.Unmarshal([]byte(reply.schema), &contract)
	if err != nil {
		t.Fatal(err)
	}

	if contract.Properties.Steps.Properties["1"].Properties.Scores.Required == nil {
		t.Fatalf("a block without concepts must require an empty list of scores, never null: %s", reply.schema)
	}

	if result.Status != checklist.StatusFail ||
		!strings.Contains(strings.Join(result.Context, "\n"), "no concept of given reaches the threshold") {
		t.Fatalf("a step of a block without concepts must fail the link: %+v", result)
	}
}

func assertRequiredBlockScores(t *testing.T, schema string) {
	t.Helper()

	var contract struct {
		Properties struct {
			Steps struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Properties struct {
						Scores struct {
							Required []string `json:"required"`
						} `json:"scores"`
					} `json:"properties"`
				} `json:"properties"`
			} `json:"steps"`
		} `json:"properties"`
	}

	err := json.Unmarshal([]byte(schema), &contract)
	if err != nil {
		t.Fatal(err)
	}

	steps := contract.Properties.Steps
	slices.Sort(steps.Required)

	if !slices.Equal(steps.Required, []string{"1", "2"}) || len(steps.Properties) != 2 {
		t.Fatalf("judge clause must be unlinked: %s", schema)
	}

	for key, required := range map[string][]string{"1": {"halve", "square"}, "2": {"result"}} {
		scores := steps.Properties[key].Properties.Scores.Required
		slices.Sort(scores)

		if !slices.Equal(scores, required) {
			t.Errorf("step %s must require precisely its block's concepts: %s", key, schema)
		}
	}
}

func TestConceptSketchEmbedsMeaningsAndInterfaceKindsWithoutArchitectureRead(t *testing.T) {
	evaluator, reply, repository := conceptRequestEvaluator(t)

	_, err := evaluator.EvaluateOne(context.Background(), &template.StoryRefineData{}, linkTestConcept,
		shippedConceptPrompt(t, repository, "sketch"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, required := range []string{"Square the entered number.", `Service interface kinds: {"calc":"ui"}`,
		"A concept does not need an architecture binding yet", "architecture bindings or helpers"} {
		if !strings.Contains(reply.user, required) {
			t.Errorf("sketch request omits %q: %s", required, reply.user)
		}
	}

	for _, forbidden := range []string{"private-architecture.yaml", "secret-binding-marker", "READ docs/architecture"} {
		if strings.Contains(reply.system+reply.user, forbidden) {
			t.Errorf("sketch must not expose architecture read paths or bindings: %s", reply.user)
		}
	}
}

func TestConceptAppliersCarryApprovedMenuChoice(t *testing.T) {
	_, reply, repository := conceptRequestEvaluator(t)

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

	for _, testCase := range []struct {
		stem    string
		subject any
	}{
		{"us-checklist", &template.StoryRefineData{}},
		{"scen-check", &template.ScenarioCheckData{}},
	} {
		reply.user = ""
		applier := validate.NewFixApplierWithPaths(reply, cfg, models,
			filepath.Join(repository, "templates", testCase.stem+".fix-applier.system.prompt.tpl"),
			filepath.Join(repository, "templates", testCase.stem+".fix-applier.prompt.tpl"))

		const approvedFix = "Widen only the approved product meaning."

		_, _ = applier.Apply(t.Context(), validate.ApplyParams{Subject: testCase.subject,
			Judgement: linkJudgement, FixPrompt: approvedFix, TmpDir: t.TempDir()})
		if !strings.Contains(reply.user, "chose apply at the menu") || !strings.Contains(reply.user, approvedFix) {
			t.Errorf("%s apply request must carry the menu approval and its fix: %s", testCase.stem, reply.user)
		}
	}
}
