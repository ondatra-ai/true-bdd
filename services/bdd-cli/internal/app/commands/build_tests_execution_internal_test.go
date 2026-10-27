package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	checklistloader "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func TestFailedPeerCannotDisappearWhenRepairUnbindsIt(t *testing.T) {
	execution, item := executionFixture(t)
	ctx := context.Background()

	writeRepair(t, `\d+`, true)

	_, err := execution.postFix(ctx, item, "")
	requireExecutionSuccess(t, err)

	if !strings.Contains(strings.Join(item.Failures, "\n"), "expected 3, got 4") {
		t.Fatalf("bound peer must first fail its own replay: %v", item.Failures)
	}

	writeRepair(t, "2", false)
	assertUnboundPeerFails(t, execution, item)
	// Repeating an unchanged repair cannot erase the peer either.
	assertUnboundPeerFails(t, execution, item)
	writeRepair(t, `\d+`, false)

	_, err = execution.postFix(ctx, item, "")
	requireExecutionSuccess(t, err)

	err = execution.finish(nil)
	if err != nil {
		t.Fatalf("peer passed its own replay: %v", err)
	}
}

func TestInitiallyBoundPeerBecomingUnboundFailsAtAttemptBound(t *testing.T) {
	execution, item := executionFixture(t)
	execution.maxFixes = 1

	writeRepair(t, "2", false)

	_, err := execution.postFix(context.Background(), item, "")
	if !errors.Is(err, ErrStepsStillFailing) || !strings.Contains(err.Error(), "calc-002-shared") {
		t.Fatalf("newly unbound peer must fail the run at the bound: %v", err)
	}
}

func assertUnboundPeerFails(t *testing.T, execution *buildTestsExecution, item *BuildTestsItem) {
	t.Helper()

	ctx := context.Background()
	_, err := execution.postFix(ctx, item, "")
	requireExecutionSuccess(t, err)

	err = execution.finish(nil)
	if !errors.Is(err, ErrStepsStillFailing) {
		t.Fatalf("unbound peer was silently forgotten: %v", err)
	}

	result, err := execution.reviewResult(ctx, item, checklist.ValidationResult{Status: checklist.StatusPass})
	if err != nil || result.Status != checklist.StatusFail {
		t.Fatalf("unbound peer must enter fix loop: %+v, %v", result, err)
	}

	evidence := strings.Join(result.Context, "\n")
	if !strings.Contains(evidence, "calc-002-shared") || !strings.Contains(evidence, "the number is 3") {
		t.Fatalf("repair evidence must name the peer and unbound step: %s", evidence)
	}
}

func executionFixture(t *testing.T) (*buildTestsExecution, *BuildTestsItem) {
	t.Helper()

	repo, err := filepath.Abs("../../../../..")
	requireExecutionSuccess(t, err)

	root := t.TempDir()
	fixture := filepath.Join(repo, "tests/bdd-cli/fixtures/build-tests-run-shared/input")
	requireExecutionSuccess(t, disk.CopyTree(root, fixture, disk.Shared))

	for _, name := range []string{"true-bdd", "templates"} {
		err := disk.CopyTree(filepath.Join(root, name), filepath.Join(repo, name), disk.Shared)
		requireExecutionSuccess(t, err)
	}

	mod, err := disk.Read(filepath.Join(root, "go.mod"))
	requireExecutionSuccess(t, err)

	mod = append(mod, []byte("\nreplace github.com/ondatra-ai/true-bdd => "+repo+"\n")...)
	requireExecutionSuccess(t, disk.Write(filepath.Join(root, "go.mod"), mod, disk.Shared))

	sum, err := disk.Read(filepath.Join(repo, "go.sum"))
	requireExecutionSuccess(t, err)

	requireExecutionSuccess(t, disk.Write(filepath.Join(root, "go.sum"), sum, disk.Shared))

	t.Chdir(root)

	cfg, err := config.NewViperConfig()
	requireExecutionSuccess(t, err)

	arch, err := architecture.Load("docs/architecture/architecture.yaml")
	requireExecutionSuccess(t, err)

	arch.Testing.Commands.Replay = "go test -tags bdd -json -count=1 ./tests/calc/"

	scenarios, err := registry.NewRegistryLoader().Load("docs/scenarios.yaml")
	requireExecutionSuccess(t, err)

	plan, err := scenariogen.BuildPlan(scenarios, arch, root, "docs/scenarios.yaml")
	requireExecutionSuccess(t, err)

	_, err = scenariogen.Write(plan, root)
	requireExecutionSuccess(t, err)

	execution := &buildTestsExecution{
		loader: checklistloader.NewChecklistLoader(cfg),
		dispatcher: testrunner.NewDispatcher(map[string]testrunner.Runner{
			testrunner.FrameworkGoTest: testrunner.NewGoTestRunner(nil),
		}),
	}
	requireExecutionSuccess(t, execution.prepare(arch, scenarios, plan, root))

	return execution, &BuildTestsItem{RegistryScenario: scenarios[0]}
}

func writeRepair(t *testing.T, pattern string, breakPeer bool) {
	t.Helper()

	body, err := disk.Read("tests/calc/steps/steps.go")
	requireExecutionSuccess(t, err)
	// Each response replaces the whole registration, as a model's repair may.
	source := string(body)
	start := strings.Index(source, "func Register")
	source = source[:start] + `func Register(suite Registrar) {
 suite.Step(` + "`^the number is (" + pattern + ")$`" + `, number)
 suite.Step(` + "`^the number is doubled$`" + `, func(s *State, _ []string) error { s.Value *= 2; return nil })
 suite.Step(` + "`^the result is (\\d+)$`" + `, result)
}
func number(state *State, args []string) error {
 value, err := strconv.Atoi(args[0]); state.Value = value; BREAK_PEER; return err
}
func result(state *State, args []string) error {
 want, err := strconv.Atoi(args[0]); if err != nil { return err }
 if state.Value != want { return fmt.Errorf("expected %d, got %d", want, state.Value) }; return nil
}
`

	fault := ""
	if breakPeer {
		fault = "if value == 3 { state.Value++ }"
	}

	source = strings.ReplaceAll(source, "BREAK_PEER", fault)
	requireExecutionSuccess(t, disk.Write("tests/calc/steps/steps.go", []byte(source), disk.Shared))
}

func requireExecutionSuccess(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func TestVerificationUsesResolvedRunningEngine(t *testing.T) {
	execution, item := executionFixture(t)
	executable, err := os.Executable()
	requireExecutionSuccess(t, err)
	executable, err = filepath.EvalSymlinks(executable)
	requireExecutionSuccess(t, err)

	execution.arch.Testing.PrebuiltEngineArg = "-engine-bin={engine_bin}"
	execution.arch.Testing.Commands.Replay = "sh verify.sh -json"
	requireExecutionSuccess(t, execution.prepare(execution.arch, execution.scenarios, execution.plan, execution.root))
	requireExecutionSuccess(t, disk.Write("expected-engine", []byte("-engine-bin="+executable), disk.Shared))
	requireExecutionSuccess(t, disk.Write("verify.sh", []byte(`#!/bin/sh
[ "$2" = "$(cat expected-engine)" ] || exit 1
exec go test -tags bdd -json -count=1 ./tests/calc/ "$3" "$4"
`), disk.Shared))
	writeRepair(t, `\d+`, false)

	_, err = execution.postFix(context.Background(), item, "")
	requireExecutionSuccess(t, err)
	requireExecutionSuccess(t, execution.finish(nil))
}
