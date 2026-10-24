package runner_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const (
	buildCodeCommand = "build code"
	confirmationTier = "high"
)

func TestMissingConfirmationRecoversOnlyBuildCode(t *testing.T) {
	for _, command := range []string{buildCodeCommand, "build tests", "us refine", "us apply", "scen check"} {
		t.Run(command, func(t *testing.T) {
			spec := confirmationSpec(t, command, applyAnswer{response: "Changed the service."})
			reruns := 0
			spec.PostNotApplied = func(_ context.Context, _ bool) (bool, error) {
				reruns++

				return true, nil
			}

			err := runner.Run(t.Context(), spec)
			if command == buildCodeCommand {
				if err != nil || reruns != 1 {
					t.Fatalf("Run = %v, reruns = %d; want success and one rerun", err, reruns)
				}
			} else if !errors.Is(err, pkgerrors.ErrFixApplierNoContent) || reruns != 0 {
				t.Fatalf("Run = %v, reruns = %d; want missing confirmation fatal", err, reruns)
			}
		})
	}
}

func TestBuildCodeProviderFailuresStayFatal(t *testing.T) {
	executionFailure := errors.New("crush exited with code 1")
	for _, testCase := range []struct {
		name   string
		answer applyAnswer
		cause  error
	}{
		{"empty output", applyAnswer{err: pkgerrors.ErrProviderProducedNoOutput("crush")}, pkgerrors.ErrProviderNoOutput},
		{"whitespace output",
			applyAnswer{response: " \n\t", err: pkgerrors.ErrProviderProducedNoOutput("crush")}, pkgerrors.ErrProviderNoOutput},
		{"nonzero exit",
			applyAnswer{response: "Changed files.", err: pkgerrors.ErrProviderExecutionFailed("crush", executionFailure)},
			executionFailure},
		{"execution error",
			applyAnswer{err: pkgerrors.ErrProviderExecutionFailed("crush", executionFailure)}, executionFailure},
		{"refused CLI",
			applyAnswer{err: pkgerrors.ErrProviderNotRegisteredForCLI("crush")}, pkgerrors.ErrProviderNotRegistered},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			spec := confirmationSpec(t, buildCodeCommand, testCase.answer)
			spec.PostNotApplied = func(_ context.Context, item bool) (bool, error) {
				t.Error("provider failure reached PostNotApplied")

				return item, nil
			}

			spec.PostFix = func(_ context.Context, item bool, _ string) (bool, error) {
				t.Error("provider failure reached PostFix")

				return item, nil
			}

			err := runner.Run(t.Context(), spec)
			if !errors.Is(err, testCase.cause) {
				t.Fatalf("Run = %v, want %v", err, testCase.cause)
			}
		})
	}
}

func TestExtractedMalformedConfirmationKeepsPostFix(t *testing.T) {
	for _, command := range []string{buildCodeCommand, "build tests", "us refine", "us apply", "scen check"} {
		t.Run(command, func(t *testing.T) {
			for _, response := range []string{"block", "no-applied"} {
				t.Run(response, func(t *testing.T) {
					spec := confirmationSpec(t, command, applyAnswer{response: response})
					applied := 0
					spec.PostFix = func(_ context.Context, _ bool, _ string) (bool, error) {
						applied++

						return true, nil
					}

					spec.PostNotApplied = func(_ context.Context, item bool) (bool, error) {
						t.Error("malformed block reached PostNotApplied")

						return item, nil
					}

					err := runner.Run(t.Context(), spec)
					if err != nil || applied != 1 {
						t.Fatalf("Run = %v, PostFix calls = %d", err, applied)
					}
				})
			}
		})
	}
}

func TestMissingConfirmationTerminalRerunCannotBypassBound(t *testing.T) {
	spec := confirmationSpec(t, buildCodeCommand, applyAnswer{response: "Changed the service."})
	reruns := 0
	spec.PostNotApplied = func(_ context.Context, _ bool) (bool, error) {
		reruns++

		return reruns == 3, nil
	}

	err := runner.Run(t.Context(), spec)
	if err == nil || !strings.Contains(err.Error(), "gave up after 3 applied fix") || reruns != 3 {
		t.Fatalf("Run = %v, reruns = %d; want terminal bound after three applies including green terminal rerun", err, reruns)
	}
}

type applyAnswer struct {
	response string
	err      error
}

func (answer applyAnswer) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string) (string, error) {
	if role == provider.RoleFix {
		return `{"fix_prompt":"Repair the service","questions":[]}`, nil
	}

	if answer.response == "block" {
		return "=== FILE_START: " + user + " ===\napplied: true\nsummary: terms: repaired\n" +
			"=== FILE_END: " + user + " ===", nil
	}

	if answer.response == "no-applied" {
		return "=== FILE_START: " + user + " ===\nsummary: repaired\n=== FILE_END: " + user + " ===", nil
	}

	return answer.response, answer.err
}

type applyUI struct{}

func (applyUI) AskQuestions(_ []engine.ClarifyQuestion) map[string]string { return nil }
func (applyUI) AskApplyRefineOrExit() engine.UserAction                   { return engine.UserActionApply }
func (applyUI) AskRefinementFeedback() string                             { return "" }
func (applyUI) DisplayFixPrompt(_ string)                                 {}

func confirmationSpec(t *testing.T, command string, answer applyAnswer) runner.Spec[bool] {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)

	for path, body := range map[string]string{
		"true-bdd/true-bdd.yaml": `paths:
  checklists_dir: checklists
engine:
  models:
    high: claude:test
    coder: crush:test
  default_prompt_model: high
  default_fix_model: high
  default_apply_model: coder
`,
		"checklists/test.yaml": `version: '1.0'
config:
  max_apply_attempts: 2
sections:
- id: verdict
  validation_prompts:
  - D: tests-pass
    F: Repair the service
`,
		"system.tpl": "Apply the fix.",
		"user.tpl":   "{{.ResultPath}}",
	} {
		writeConfirmationFile(t, path, body)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	defaults := map[provider.Role]string{
 provider.RolePrompt: confirmationTier, provider.RoleFix: confirmationTier, provider.RoleApply: "coder",
 }

	models, err := provider.NewRegistry(
 map[string]string{confirmationTier: "claude:test", "coder": "crush:test"}, defaults,
 )
	if err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(root, "tmp")

	err = disk.Dir(tmp, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	return runner.Spec[bool]{
		Name: command, ChecklistName: "test", Fix: true, TmpDir: tmp,
		LoadItems:  func(context.Context) ([]bool, error) { return []bool{false}, nil },
		GetSubject: func(bool) (string, string) { return "failing-set", "Failing set" },
		Checks: checks.Set[bool]{"tests-pass": {Run: func(passed bool, _ checks.Input) (checks.Verdict, error) {
			return checks.Verdict{Pass: passed}, nil
		}}},
		Evaluator:       validate.NewChecklistEvaluatorWithPaths(answer, cfg, models, "system.tpl", "user.tpl"),
		FixGenerator:    validate.NewFixPromptGeneratorWithPaths(answer, cfg, models, "system.tpl", "user.tpl"),
		FixApplier:      validate.NewFixApplierWithPaths(answer, cfg, models, "system.tpl", "user.tpl"),
		ChecklistLoader: checklist.NewChecklistLoader(cfg), DocResolver: docs.NewResolver(cfg),
		Renderer: runner.NewTableRenderer(), UI: applyUI{},
		PostFix:  func(_ context.Context, _ bool, _ string) (bool, error) { return true, nil },
		Finalize: func(*engine.Result[bool]) error { return nil },
	}
}

func writeConfirmationFile(t *testing.T, path, body string) {
	t.Helper()

	err := disk.Dir(filepath.Dir(path), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write(path, []byte(body), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}
