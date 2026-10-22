package testrunner_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func TestSelectedScenarioRequiresPassInOwningPackage(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name     string
		selected string
		wantErr  bool
	}{
		{"skipped", `func TestShared(t *testing.T) { t.Skip("selected Scenario did not run") }`, true},
		{"excluded", `func TestOther(t *testing.T) {}`, true},
		{"passed", `func TestShared(t *testing.T) {}`, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			files := map[string]string{
				"go.mod":                    "module fixture/selected\n\ngo 1.26\n",
				"selected/scenario_test.go": "package selected_test\nimport \"testing\"\n" + testCase.selected,
				"other/scenario_test.go":    "package other_test\nimport \"testing\"\nfunc TestShared(t *testing.T) {}\n",
			}
			for name, body := range files {
				err := disk.Write(filepath.Join(root, name), []byte(body), disk.Shared)
				if err != nil {
					t.Fatal(err)
				}
			}

			cfg := testrunner.Config{
				Framework: testrunner.FrameworkGoTest, ConfigFile: filepath.Join(root, "go.mod"),
				Command: "go test -json ./...", Tests: []string{"fixture/selected/selected::TestShared"},
			}

			_, err := testrunner.NewGoTestRunner(nil).Discover(context.Background(), cfg, "", "")
			if testCase.wantErr {
				if !errors.Is(err, testrunner.ErrScenarioNotRun) {
					t.Fatalf("want skipped/missing selected Scenario failure: %v", err)
				}
			} else if err != nil {
				t.Fatalf("selected Scenario passed in its owning package: %v", err)
			}
		})
	}
}
