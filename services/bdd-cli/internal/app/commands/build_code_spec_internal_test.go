package commands

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func TestCodeFailureMatchesGeneratedNameAndOwningPackage(t *testing.T) {
	const (
		matchedName     = "host/tests/calc::TestCalc001Double"
		fallbackService = "sole-service"
	)

	root := t.TempDir()
	t.Chdir(root)

	for name, content := range map[string]string{
		"go.mod":                 "module host\n",
		"true-bdd/true-bdd.yaml": "documents:\n  scenarios_yaml: docs/scenarios.yaml\n",
		"docs/scenarios.yaml": `scenarios:
  calc-001-double:
    service: calculator
    path: tests/calc/calc_test.go
    merged_steps:
      given: [a number]
      when: [it doubles]
      then: [the result]
`,
	} {
		err := disk.Write(filepath.Join(root, name), []byte(content), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	deps := BuildCodeDeps{DocResolver: docs.NewResolver(cfg)}

	for _, example := range []struct {
		name, framework string
		matched         bool
	}{
		{matchedName, testrunner.FrameworkGoTest, true},
		{"host/tests/other::TestCalc001Double", testrunner.FrameworkGoTest, false},
		{"host/tests/calc::TestOther", testrunner.FrameworkGoTest, false},
		{"host/tests/calc::<build>", testrunner.FrameworkGoTest, false},
		{matchedName, "jest", false},
		{matchedName, "playwright", false},
	} {
		t.Run(example.framework+example.name, func(t *testing.T) {
			failure := &testrunner.FailingTest{TestName: example.name, Framework: example.framework}
			arch := &architecture.Architecture{Services: []architecture.Service{{Name: fallbackService}, {Name: "second"}}}
			specifyCodeFailures(deps, arch, "", []*testrunner.FailingTest{failure})

			if (failure.Scenario != nil) != example.matched {
				t.Fatalf("matched Scenario: got %v, want %v", failure.Scenario, example.matched)
			}

			wantService := ""
			if example.matched {
				wantService = "calculator"
			}

			if failure.Service != wantService {
				t.Fatalf("service: got %q, want %q", failure.Service, wantService)
			}

			if !example.matched {
				arch.Services = arch.Services[:1]
				specifyCodeFailures(deps, arch, "", []*testrunner.FailingTest{failure})

				if failure.Service != fallbackService {
					t.Fatalf("only-service fallback: got %q", failure.Service)
				}
			}
		})
	}
}
