package commands_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	runfs "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

func TestBuildCodeRefusesAnUnreadableRegistryWithoutCodeBeforeAnyTurn(t *testing.T) {
	const registryDirectory = "directory"
	for _, kind := range []string{"malformed", "absent", registryDirectory} {
		t.Run(kind, func(t *testing.T) {
			stepsDeps := greenCodeHost(t)
			reply := &conceptReplies{}
			deps := redCodeDependencies(t, reply, stepsDeps)

			var err error

			switch kind {
			case "malformed":
				err = disk.Write("docs/scenarios.yaml", []byte("scenarios: [\n"), disk.Shared)
			case "absent", registryDirectory:
				err = disk.Remove("docs/scenarios.yaml")
			}

			if err != nil {
				t.Fatal(err)
			}

			if kind == registryDirectory {
				err = disk.Dir("docs/scenarios.yaml", disk.Shared)
				if err != nil {
					t.Fatal(err)
				}
			}

			err = commands.RunBuildCode(t.Context(), deps, "docs/architecture/architecture.yaml", true)
			if !errors.Is(err, commands.ErrRedCheck) || !strings.Contains(err.Error(), "docs/scenarios.yaml") {
				t.Fatalf("want red-check Registry refusal naming its document, got %v", err)
			}

			if len(reply.requests) != 0 {
				t.Fatalf("%d AI turns before Registry refusal", len(reply.requests))
			}
		})
	}
}

func TestBuildCodeStopsOnAGreenScenarioWithoutCodeBeforeAnyTurn(t *testing.T) {
	for _, built := range []bool{false, true} {
		t.Run(map[bool]string{false: "no code", true: "code present"}[built], func(t *testing.T) {
			stepsDeps := greenCodeHost(t)

			if built {
				err := disk.Write("services/calc/main.go", []byte("package main"), disk.Private)
				if err != nil {
					t.Fatal(err)
				}
			}

			reply := &conceptReplies{}
			deps := redCodeDependencies(t, reply, stepsDeps)

			err := commands.RunBuildCode(t.Context(), deps, "docs/architecture/architecture.yaml", false)
			if built {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "scenario green without target") ||
				!strings.Contains(err.Error(), "calc-001-double") {
				t.Fatalf("want green-without-code refusal naming its Scenario, got %v", err)
			}

			if len(reply.requests) != 0 {
				t.Fatalf("%d AI turns before red check", len(reply.requests))
			}
		})
	}
}

func TestBuildCodeKeepsTheRegistryOptionalWithoutAStepsPackage(t *testing.T) {
	stepsDeps := greenCodeHost(t)
	reply := &conceptReplies{}
	deps := redCodeDependencies(t, reply, stepsDeps)

	err := disk.RemoveTree("tests")
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("tests/calc/plain_test.go", []byte("package calc_test\nimport \"testing\"\n"+
		"func TestHandwritten(t *testing.T) {}\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Dir("tests/calc/fixtures/nested/steps", disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("docs/scenarios.yaml", []byte("scenarios: [\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildCode(t.Context(), deps, "docs/architecture/architecture.yaml", false)
	if err != nil {
		t.Fatalf("hand-written Go suite refused: %v", err)
	}

	if len(reply.requests) != 0 {
		t.Fatalf("%d AI turns for a green hand-written suite", len(reply.requests))
	}
}

func greenCodeHost(t *testing.T) commands.BuildStepsDeps {
	t.Helper()

	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	stepsDeps := buildFixture(t)

	err = disk.CopyTree(".", filepath.Join(repository, "tests/true-bdd/fixtures/build-steps-wiring/input"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	module := "module fixture/buildtests\n\ngo 1.25.0\n\n" +
		"require (\n github.com/ondatra-ai/true-bdd/pkg v0.0.0\n gopkg.in/yaml.v3 v3.0.1 // indirect\n)\n" +
		"replace github.com/ondatra-ai/true-bdd/pkg => " + filepath.Join(repository, "pkg") + "\n"

	err = disk.Write("go.mod", []byte(module), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Copy("go.sum", filepath.Join(repository, "pkg/go.sum"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.RemoveTree("services")
	if err != nil {
		t.Fatal(err)
	}

	source := "package steps\nfunc Register(suite Registrar) {\n" +
		"suite.Step(`^.*$`, func(*State, []string) error { return nil })\n}\n"

	err = disk.Write("tests/calc/steps/register.go", []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildTests(t.Context(), commands.BuildTestsDeps{RegistryLoader: stepsDeps.RegistryLoader},
		"docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	return stepsDeps
}

func redCodeDependencies(t *testing.T, reply *conceptReplies,
	stepsDeps commands.BuildStepsDeps,
) commands.BuildCodeDeps {
	t.Helper()

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	runDir, err := runfs.NewRunDirectory("tmp")
	if err != nil {
		t.Fatal(err)
	}

	const modelRef = "claude:test"

	models, err := provider.NewRegistry(map[string]string{groupTestTier: modelRef, "coder": modelRef},
		map[provider.Role]string{
			provider.RolePrompt: groupTestTier, provider.RoleFix: groupTestTier, provider.RoleApply: groupTestTier,
		})
	if err != nil {
		t.Fatal(err)
	}

	return commands.BuildCodeDeps{
		DocResolver: docs.NewResolver(cfg), ChecklistLoader: stepsDeps.ChecklistLoader,
		TestRunnerDispatcher: testrunner.NewDispatcher(map[string]testrunner.Runner{
			testrunner.FrameworkGoTest: testrunner.NewGoTestRunner(nil),
		}),
		BuildCodeEvaluator:          validate.NewChecklistEvaluator(reply, cfg, models),
		BuildCodeFixPromptGenerator: validate.NewFixPromptGenerator(reply, cfg, models),
		BuildCodeFixApplier:         validate.NewFixApplier(reply, cfg, models),
		TableRenderer:               runner.NewTableRenderer(), RunDir: runDir,
	}
}
