package architecture_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const modelTestService = "sample"

func TestModelsRequireResourcesAndAScenarioOfTheirService(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		"architecture.yaml": "architecture:\n  services:\n  - name: sample\n    models:\n      sample_checklist:\n" +
			"        templates: {system: system.tpl, user: user.tpl}\n        scheme: model.yaml\n",
		"system.tpl": "System", "user.tpl": "{{.Question}}", "model.yaml": "type: object\n",
	} {
		err := disk.Write(filepath.Join(root, path), []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, testCase := range []struct {
		name  string
		steps map[string][]string
		want  bool
	}{
		{"named", map[string][]string{modelTestService: {"the sample_checklist prompt holds its question verbatim"}}, true},
		{"not named", nil, false},
		{"substring", map[string][]string{modelTestService: {"the other_sample_checklist prompt"}}, false},
		{"another service", map[string][]string{"other": {"sample_checklist"}}, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := architecture.CheckModels(filepath.Join(root, "architecture.yaml"), root, testCase.steps)
			if testCase.want {
				if err != nil {
					t.Fatal(err)
				}

				return
			}

			if !errors.Is(err, architecture.ErrModelDeclaration) || !strings.Contains(err.Error(), "sample_checklist") {
				t.Fatalf("want named Model declaration refusal, got %v", err)
			}
		})
	}

	assertMissingModelTemplate(t, root)
}

func assertMissingModelTemplate(t *testing.T, root string) {
	t.Helper()

	err := disk.Remove(filepath.Join(root, "user.tpl"))
	if err != nil {
		t.Fatal(err)
	}

	err = architecture.CheckModels(filepath.Join(root, "architecture.yaml"), root, nil)
	if !errors.Is(err, architecture.ErrModelDeclaration) || !strings.Contains(err.Error(), "user.tpl") {
		t.Fatalf("want missing template refusal, got %v", err)
	}
}

func TestModelNamesAreLiteralScenarioWords(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for path, content := range map[string]string{
		"architecture.yaml": "architecture:\n  services:\n  - name: sample\n    models:\n      sample-checklist:\n" +
			"        templates: {system: model.tpl, user: model.tpl}\n        scheme: model.yaml\n",
		"model.tpl": "{{.Question}}", "model.yaml": "type: object\n",
	} {
		err := disk.Write(filepath.Join(root, path), []byte(content), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := architecture.CheckModels(filepath.Join(root, "architecture.yaml"), root,
		map[string][]string{modelTestService: {"the sample-checklist prompt keeps its question"}})
	if err != nil {
		t.Fatal(err)
	}
}
