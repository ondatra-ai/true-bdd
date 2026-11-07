package validate_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestStepArgumentScenarioPromptsBeforeRecording(t *testing.T) {
	for _, fixture := range []string{"step-arguments-present", "step-arguments-absent"} {
		t.Run(fixture, func(t *testing.T) {
			repository := argumentFixtureHost(t, fixture)

			cfg, err := config.NewViperConfig()
			if err != nil {
				t.Fatal(err)
			}

			reply := &linkReply{response: `{"answer":"pass","context":["ok"]}`}
			evaluator := validate.NewChecklistEvaluatorWithPaths(reply, cfg, argumentModels(t),
				"templates/scen-check.checklist.system.prompt.tpl",
 "templates/scen-check.checklist.prompt.tpl", "scen_check_checklist")

			source, err := disk.Read("true-bdd/checklists/scen-check.yaml")
			if err != nil {
				t.Fatal(err)
			}

			var document checklist.Checklist

			err = yaml.Unmarshal(source, &document)
			if err != nil {
				t.Fatal(err)
			}

			for _, section := range document.Sections {
				if section.ID != "scenario_quality" {
					continue
				}

				_, err = evaluator.EvaluateOne(t.Context(), argumentFixtureScenario(t), "E2E-001",
					checklist.PromptWithContext{Index: 1, SectionID: "scen-check/" + section.ID,
						Prompt: section.ValidationPrompts[0]}, "tmp/run")
				if err != nil {
					t.Fatal(err)
				}
			}

			assertArgumentFixtureRules(t, filepath.Join(repository, "tests/true-bdd/fixtures", fixture), reply)
		})
	}
}

func argumentFixtureHost(t *testing.T, fixture string) string {
	t.Helper()

	repository, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(t.TempDir())

	for target, source := range map[string]string{
		"true-bdd": "true-bdd", "templates": "templates", ".": "tests/true-bdd/fixtures/" + fixture + "/input"} {
		err = disk.CopyTree(target, filepath.Join(repository, source), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	return repository
}

func argumentFixtureScenario(t *testing.T) *template.ScenarioCheckData {
	t.Helper()

	scenarios, err := registry.NewRegistryLoader().Load("docs/scenarios.yaml")
	if err != nil {
		t.Fatal(err)
	}

	scenario := scenarios[0]

	item := &template.ScenarioCheckData{ID: scenario.ID, Description: scenario.Description, Service: scenario.Service,
		Path: scenario.Path, Steps: scenario.Steps}
	for _, ref := range scenario.UserStories {
		item.UserStories = append(item.UserStories, template.StoryRef{Story: ref.Story, ScenarioID: ref.ScenarioID})
	}

	return item
}

func assertArgumentFixtureRules(t *testing.T, fixture string, reply *linkReply) {
	t.Helper()

	data, err := disk.Read(filepath.Join(fixture, "model-expectations.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var rules map[string]struct {
 Glob string `yaml:"glob"`
 Pattern string `yaml:"pattern"`
 }

	err = yaml.Unmarshal(data, &rules)
	if err != nil {
		t.Fatal(err)
	}

	for name, rule := range rules {
		pattern, compileErr := regexp.Compile(rule.Pattern)
		if compileErr != nil {
			t.Fatal(compileErr)
		}

		request := reply.user
		if strings.Contains(rule.Glob, "-system.txt") {
			request = reply.system
		}

		if !pattern.MatchString(request) {
			t.Fatalf("%s no longer holds before recording:\n%s", name, request)
		}
	}
}
