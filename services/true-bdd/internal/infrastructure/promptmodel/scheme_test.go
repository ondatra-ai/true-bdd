package promptmodel_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/promptmodel"
	"gopkg.in/yaml.v3"
)

func TestChecklistModelSchemes(t *testing.T) {
	t.Parallel()

	for name, subject := range map[string]string{
		"us_refine_checklist": "{id: example, title: Example, as_a: Owner, i_want: examples, " +
			"so_that: I can compare, status: draft, acceptance_criteria: []}",
		"us_apply_checklist": "{StoryID: example, StoryPath: story.yaml, ACID: 001-example, " +
			"LineageScenarioID: example-001-example, Description: Example, " +
			"RequirementsScratchPath: tmp/registry.yaml, Steps: {Given: [], When: [], " +
			"Then: []}}",
		"scen_check_checklist": "{ID: example-001-example, Description: Example, Service: example, " +
			"Path: tests/example/example_test.go, Steps: {Given: [], When: [], Then: []}, " +
			"UserStories: []}",
		"build_steps_checklist": "{Suite: tests/example, Label: example, Concept: {Means: Compare an example.}}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := promptmodel.Load(name, filepath.Join("../../../../../true-bdd/models", name+".schema.yaml"))
			if err != nil {
				t.Fatal(err)
			}

			for _, defect := range []string{"valid", "missing", "mistyped", "subject"} {
				t.Run(defect, func(t *testing.T) { checkModelInstance(t, scheme, name, subject, defect) })
			}
		})
	}
}

func checkModelInstance(t *testing.T, scheme *promptmodel.Scheme, name, subject, defect string) {
	t.Helper()

	var data map[string]any

	source := "Question: Compare this.\nRationale: Preserve behaviour.\nFixTemplate: Repair it.\n" +
		"ResultPath: tmp/result.yaml\nStructured: true\nDocs: {product: {FilePath: docs/product.yaml}}\nSubject: " + subject

	err := yaml.Unmarshal([]byte(source), &data)
	if err != nil {
		t.Fatal(err)
	}

	field := "Question"

	switch defect {
	case "missing":
		delete(data, field)
	case "mistyped":
		data[field] = false
	case "subject":
		field = "Subject"
		data[field] = "wrong shape"
	}

	err = scheme.Validate(data)
	if defect == "valid" {
		if err != nil {
			t.Fatal(err)
		}

		return
	}

	if !errors.Is(err, promptmodel.ErrModelData) || !strings.Contains(err.Error(), name) ||
		!strings.Contains(err.Error(), field) {
		t.Fatalf("want distinct %s/%s mismatch, got %v", name, field, err)
	}
}
