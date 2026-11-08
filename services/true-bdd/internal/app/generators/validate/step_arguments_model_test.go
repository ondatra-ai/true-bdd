package validate_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func argumentModels(t *testing.T) *provider.Registry {
	t.Helper()

	models, err := provider.NewRegistry(map[string]string{linkTestTier: linkTestModel}, map[provider.Role]string{
		provider.RolePrompt: linkTestTier, provider.RoleFix: linkTestTier, provider.RoleApply: linkTestTier})
	if err != nil {
		t.Fatal(err)
	}

	return models
}

func TestChecklistModelsTransportCompleteStepArguments(t *testing.T) {
	for stem := range stepArgumentTemplateSubjects() {
		if stem == argumentCodeStem {
			continue
		}

		for _, byPath := range []bool{false, true} {
			t.Run(stem+map[bool]string{false: "/embedded", true: "/path"}[byPath], func(t *testing.T) {
				checkArgumentModel(t, stem, byPath)
			})
		}
	}
}

func checkArgumentModel(t *testing.T, stem string, byPath bool) {
	t.Helper()

	_, reply, repository := conceptRequestEvaluator(t)
	declareArgumentSection(t)

	modelName := map[string]string{argumentStoryStem: "us_refine_checklist", argumentApplyStem: "us_apply_checklist",
		argumentScenarioStem: "scen_check_checklist", argumentStepsStem: "build_steps_checklist"}[stem]

	body, err := disk.Read(filepath.Join(repository, "true-bdd/models", modelName+".schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write(filepath.Join("true-bdd/models", modelName+".schema.yaml"), body, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	base := stem
	if stem == argumentStepsStem || stem == argumentScenarioStem {
		base += ".checklist"
	}

	evaluator := validate.NewChecklistEvaluatorWithPaths(reply, cfg, argumentModels(t),
		filepath.Join(repository, "templates", base+".system.prompt.tpl"),
		filepath.Join(repository, "templates", base+".prompt.tpl"), modelName)

	prompt := checklist.PromptWithContext{Index: 1, Prompt: checklist.Prompt{Question: argumentQuestion}}
	if byPath {
		prompt.Prompt.Docs = []string{argumentArchitectureKey}
	}

	_, err = evaluator.EvaluateOne(t.Context(), stepArgumentTemplateSubjects()[stem],
		"argument-item", prompt, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	reference := ""
	if byPath {
		reference = argumentArchitecturePath
	}

	requireArgumentEvidence(t, reply.user, reference)
}

func TestAbsentArgumentsPreserveAnOlderHostModelScheme(t *testing.T) {
	_, reply, repository := conceptRequestEvaluator(t)

	const modelName = "us_apply_checklist"

	source, err := disk.Read(filepath.Join(repository, "true-bdd/models", modelName+".schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	source = []byte(strings.Replace(string(source), "  StepArguments: *id001\n", "", 1))

	err = disk.Write(filepath.Join("true-bdd/models", modelName+".schema.yaml"), source, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	evaluator := validate.NewChecklistEvaluatorWithPaths(reply, cfg, argumentModels(t),
		filepath.Join(repository, "templates/us.apply-checklist.system.prompt.tpl"),
		filepath.Join(repository, "templates/us.apply-checklist.prompt.tpl"), modelName)

	_, err = evaluator.EvaluateOne(t.Context(), stepArgumentTemplateSubjects()[argumentApplyStem], "argument-item",
		checklist.PromptWithContext{Index: 1, Prompt: checklist.Prompt{Question: argumentQuestion}}, t.TempDir())
	if err != nil {
		t.Fatalf("host without section no longer accepts its original scheme: %v", err)
	}

	if strings.Contains(reply.user, "Step arguments") {
		t.Fatalf("absent section changed prompt: %s", reply.user)
	}
}
