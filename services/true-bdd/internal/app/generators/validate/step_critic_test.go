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
	system string
	user   string
	schema string
	answer string
}

func (a *critiqueAI) ExecutePromptWithSystem(_ context.Context, role provider.Role, system, user string,
	model provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	a.model, a.role, a.system, a.user, a.schema = model, role, system, user, schema

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

// A concept host's critique judges each AI-proposed pattern as well as the
// body, against the binding and the fixture inputs; a template host keeps
// its rule that the engine's pattern is never judged.
func TestConceptCritiqueJudgesPatternsAndFixtureContracts(t *testing.T) {
	t.Parallel()

	concept, phrase := critiqueRequest(t, true), critiqueRequest(t, false)
	requireTerms(t, concept.system, "pattern_captures", "pattern_scope", "fixture inputs")
	requireTerms(t, concept.schema, "pattern_captures", "pattern_scope")
	requireTerms(t, concept.user, "fixture inputs")

	if strings.Contains(concept.system, "Never judge them") || !strings.Contains(phrase.system, "Never judge them") {
		t.Errorf("only a template host keeps the engine's pattern out of the critique:\n%s", concept.system)
	}

	for _, leak := range []string{"Catalogue", "Phrase mapping", "template"} {
		if strings.Contains(concept.system+concept.user, leak) {
			t.Errorf("concept critique carries the template form %q", leak)
		}
	}

	if concept.schema == phrase.schema || strings.Contains(phrase.schema, "pattern_") {
		t.Errorf("template schema changed or concept schema lacks pattern categories:\n%s", phrase.schema)
	}
}

// critiqueRequest renders the shipped critique templates for one host form.
func critiqueRequest(t *testing.T, concept bool) *critiqueAI {
	t.Helper()

	repository, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{linkTestTier: "claude:default"},
		map[provider.Role]string{provider.RolePrompt: linkTestTier, provider.RoleFix: linkTestTier,
			provider.RoleApply: linkTestTier})
	if err != nil {
		t.Fatal(err)
	}

	client := &critiqueAI{answer: `{"answer":"pass","findings":[]}`}
	critic := validate.NewStepCritic(client, models,
		filepath.Join(repository, "templates/build-steps.critique.system.prompt.tpl"),
		filepath.Join(repository, "templates/build-steps.critique.prompt.tpl"))

	_, err = critic.Critique(context.Background(), validate.StepCritiqueData{
		Definition: "halving", Context: "context", Sources: "sources", Concept: concept,
	}, "", filepath.Join(t.TempDir(), "turn"))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func requireTerms(t *testing.T, text string, terms ...string) {
	t.Helper()

	for _, term := range terms {
		if !strings.Contains(text, term) {
			t.Errorf("lacks %q:\n%s", term, text)
		}
	}
}
