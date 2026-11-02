package architecture_test

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func TestSpecificationNamesEveryDeclaredModelInAShortScenarioStep(t *testing.T) {
	t.Parallel()

	const maximumRuleCharacters = 200

	root := "../../../../.."

	scenarios, err := registry.NewRegistryLoader().Load(filepath.Join(root, "docs/scenarios.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	serviceSteps := make(map[string][]string)

	for _, scenario := range scenarios {
		for _, step := range scenario.CheckedSteps() {
			serviceSteps[scenario.Service] = append(serviceSteps[scenario.Service], step.Text)
			if strings.Contains(step.Text, "_checklist") && utf8.RuneCountInString(step.Text) > maximumRuleCharacters {
				t.Errorf("%s: Model rule exceeds %d characters: %s", scenario.ID, maximumRuleCharacters, step.Text)
			}
		}
	}

	err = architecture.CheckModels(filepath.Join(root, "docs/architecture/architecture.yaml"), root, serviceSteps)
	if err != nil {
		t.Fatal(err)
	}
}
