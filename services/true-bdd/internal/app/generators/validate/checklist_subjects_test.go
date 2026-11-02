package validate_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestChecklistModelsAcceptTheirTypedSubjects(t *testing.T) {
	const example = "example"

	repository, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	t.Chdir(root)

	err = disk.Write("true-bdd/true-bdd.yaml", []byte("templates: {}\n"), disk.Shared)
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

	for _, sample := range []struct {
		name, base string
		subject    any
	}{
		{"us_refine_checklist", "us-checklist", &template.StoryRefineData{Story: story.Story{ID: example}}},
		{"us_apply_checklist", "us.apply-checklist", &template.ScenarioApplyData{StoryID: example}},
		{"scen_check_checklist", "scen-check.checklist", &template.ScenarioCheckData{ID: example}},
		{"build_steps_checklist", "build-steps.checklist", &commands.BuildStepsItem{
			Suite: "tests/example", Concept: &commands.BuildStepsConcept{Name: example, Means: "Compare examples."},
		}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := disk.Read(filepath.Join(repository, "true-bdd/models", sample.name+".schema.yaml"))
			if err != nil {
				t.Fatal(err)
			}

			err = disk.Write(filepath.Join("true-bdd/models", sample.name+".schema.yaml"), source, disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			reply := &linkReply{response: `{"answer":"pass","context":[]}`}
			evaluator := validate.NewChecklistEvaluatorWithPaths(reply, cfg, models,
				filepath.Join(repository, "templates", sample.base+".system.prompt.tpl"),
				filepath.Join(repository, "templates", sample.base+".prompt.tpl"), sample.name)

			_, err = evaluator.EvaluateOne(t.Context(), sample.subject, example, checklist.PromptWithContext{
				Index: 1, Prompt: checklist.Prompt{Question: "Compare this."},
			}, root)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(reply.user, example) {
				t.Fatalf("typed Subject was not rendered: %s", reply.user)
			}
		})
	}
}
