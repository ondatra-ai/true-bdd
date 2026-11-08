package scenariogen_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func TestFolderRequiresAQuotedFixtureNameAnywhereInAStep(t *testing.T) {
	t.Parallel()

	for _, text := range []string{`stdout matches ^.*"offline".*$`, `the offline toolchain`} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			plan, root := wiringPlan(t)

			_, err := scenariogen.Write(plan, root)
			if err != nil {
				t.Fatal(err)
			}

			err = disk.Dir(filepath.Join(root, "checks/arithmetic/fixtures/offline"), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			scenarios := []*registry.RegistryScenario{
				{ID: "calc-001", Path: "checks/arithmetic/calc_test.go",
					Statements: []registry.Statement{{Text: text}}},
			}

			err = scenariogen.CheckFolder(scenarios, root)
			if strings.Contains(text, `"offline"`) {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "fixture tree") {
				t.Fatalf("want orphan fixture refusal, got %v", err)
			}
		})
	}
}
