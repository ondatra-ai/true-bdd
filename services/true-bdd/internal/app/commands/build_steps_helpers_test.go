package commands_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/git"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
)

// Supplied replies pin the engine flow; Claude runs the live proof in drill round 8 on cut B after #296.
func TestBuildStepsFixWritesCompilesAndCritiquesAFreshNarrowingHelper(t *testing.T) {
	for _, moveStateMethod := range []bool{false, true} {
		t.Run(fmt.Sprintf("move-State-method-%t", moveStateMethod), func(t *testing.T) {
			reply := &narrowingWriter{t: t}
			deps := narrowingHost(t, reply, moveStateMethod)

			err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
			if err != nil {
				t.Fatal(err)
			}

			if reply.writes != 1 || !strings.Contains(reply.critique, "tests/calc/helpers/narrow.go") ||
				!strings.Contains(reply.critique, "func NarrowChecklists(") ||
				!strings.Contains(reply.critique, "strings.Contains") {
				t.Fatalf("helper missed body writing or critique: writes=%d, critique=%s", reply.writes, reply.critique)
			}
			// A second inventory needs no AI and proves whole-suite Step coverage remains complete.
			err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
			if err != nil {
				t.Fatal(err)
			}

			runNarrowedDefinition(t, reply.definition)
		})
	}
}

type narrowingWriter struct {
	t          *testing.T
	definition stepskeleton.Definition
	critique   string
	writes     int
}

func (reply *narrowingWriter) ExecutePromptWithSystem(_ context.Context, role provider.Role, system, user string,
	_ provider.ModelRef, mode provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	switch {
	case strings.Contains(schema, `"bindings"`):
		return `{"bindings":{"given:project tree":{"unambiguous":true,"reason":"Narrows the declared checklist."}}}`, nil
	case role == provider.RoleFix:
		return `{"fix_prompt":"Write the narrowing helper and call it from the selected body.","questions":[]}`, nil
	case role == provider.RoleApply:
		reply.write(mode)

		marker := "=== FILE_START: "

		start := strings.Index(system, marker)
		if start < 0 {
			return "", errors.New("apply prompt lacks confirmation path")
		}

		path, _, _ := strings.Cut(system[start+len(marker):], " ===")

		return marker + path + " ===\napplied: true\n=== FILE_END: " + path + " ===", nil
	default:
		reply.critique = user

		return `{"answer":"pass","findings":[]}`, nil
	}
}

func (reply *narrowingWriter) write(mode provider.ExecutionMode) {
	reply.t.Helper()

	reply.writes++
	if !slices.Contains(mode.SourceWriteRoots, "tests/calc/helpers/**") {
		reply.t.Fatalf("writer lacks helper root: %+v", mode)
	}

	if reply.writes != 1 {
		reply.t.Fatal("fresh helper did not converge on the first apply")
	}

	_, err := os.Stat("tests/calc/helpers/narrow.go")
	if !os.IsNotExist(err) {
		reply.t.Fatal("helper already exists")
	}

	writeNarrowingFile(reply.t, "tests/calc/helpers/narrow.go", narrowingHelperSource)
	body := fmt.Sprintf("package steps\nimport \"fixture/narrow/tests/calc/helpers\"\n"+
		"//truebdd:concept given \"project tree\"\nfunc %s(state *State, args []string) error {\n"+
		"return helpers.NarrowChecklists(state.Path, args[0])\n}\n", reply.definition.Function)
	writeNarrowingFile(reply.t, reply.definition.File, body)
	writeNarrowingFile(reply.t, "tests/calc/steps/state.go", "package steps\ntype State struct{ Path string }\n")
}

func narrowingHost(t *testing.T, reply *narrowingWriter, moveStateMethod bool) commands.BuildStepsDeps {
	t.Helper()

	deps := buildFixture(t)
	for path, source := range narrowingHostFiles() {
		writeNarrowingFile(t, path, source)
	}

	if moveStateMethod {
		writeNarrowingFile(t, "tests/calc/steps/state.go", "package steps\ntype State struct{ Path string }\n"+
			"func (s *State) NarrowChecklists(string) error { return nil }\n")
	}

	definition, err := stepskeleton.WriteConcept("tests/calc/steps", "given", "project tree",
		`^the checklist is narrowed to "([^"]+)"$`)
	if err != nil {
		t.Fatal(err)
	}

	reply.definition = definition

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models := testModels(t)
	deps.Evaluator = validate.NewChecklistEvaluator(reply, cfg, models)
	deps.FixGenerator = validate.NewFixPromptGeneratorWithPaths(reply, cfg, models,
		"templates/build-steps.fix-generator.system.prompt.tpl", "templates/build-steps.fix-generator.prompt.tpl")
	deps.FixApplier = validate.NewFixApplierWithPaths(reply, cfg, models,
		"templates/build-steps.fix-applier.system.prompt.tpl", "templates/build-steps.fix-applier.prompt.tpl")
	deps.Critic = validate.NewStepCritic(reply, models,
		"templates/build-steps.critique.system.prompt.tpl", "templates/build-steps.critique.prompt.tpl")
	deps.TableRenderer = runner.NewTableRenderer()
	deps.TmpDir = t.TempDir()
	useNarrowingAnswer(t)

	deps.UserInputCollector = input.NewUserInputCollector()

	return deps
}

func useNarrowingAnswer(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "answers")
	writeNarrowingFile(t, path, "1\n")

	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	previous := os.Stdin //nolint:forbidigo // temporarily supplies this process's fix-menu input.
	os.Stdin = reader    //nolint:forbidigo // restored before the next test.

	t.Cleanup(func() { os.Stdin = previous; _ = reader.Close() }) //nolint:forbidigo // restores the process descriptor.
}

func writeNarrowingFile(t *testing.T, path, source string) {
	t.Helper()

	err := disk.Write(path, []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func runNarrowedDefinition(t *testing.T, definition stepskeleton.Definition) {
	t.Helper()

	source := fmt.Sprintf(`package steps
import ("os"; "path/filepath"; "strings"; "testing")
func TestNarrowing(t *testing.T) {
 path := filepath.Join(t.TempDir(), "checklist.yaml")
 checklist := "sections:\n- validation_prompts:\n  - Q: {prompt: Keep this}\n  - Q: {prompt: Drop this}\n"
 if err := os.WriteFile(path, []byte(checklist), 0600); err != nil { t.Fatal(err) }
 if err := %s(&State{Path:path}, []string{"Keep"}); err != nil { t.Fatal(err) }
 data, err := os.ReadFile(path)
 if err != nil || !strings.Contains(string(data), "Keep this") || strings.Contains(string(data), "Drop this") {
  t.Fatalf("narrowed checklist: %%s %%v", data, err)
 }
}
`, definition.Function)
	writeNarrowingFile(t, "tests/calc/steps/narrow_test.go", source)
	cmd := exec.CommandContext(t.Context(), "go", "test", "./tests/calc/steps", "-run", "TestNarrowing", "-count=1")

	cmd.Env = append(git.ScrubEnvironment(os.Environ()), "GOWORK=off")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated narrowing body: %s: %v", out, err)
	}
}
