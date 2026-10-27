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
	usApplyCommand    = "us apply"
	usRefineCommand   = "us refine"
	scenCheckCommand  = "scen check"
	buildCodeCommand  = "build code"
	confirmationTier  = "high"
	buildTestsCommand = "build tests"
	blockAnswer       = "block"
)

func TestMissingConfirmationRecoversOnlyBuildCode(t *testing.T) {
	for _, command := range []string{
		buildCodeCommand, buildTestsCommand, usRefineCommand, usApplyCommand, scenCheckCommand,
	} {
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

func TestBuildCodeUnclassifiedProviderFailuresStayFatal(t *testing.T) {
	executionFailure := errors.New("codex exited with code 1")
	for _, testCase := range []struct {
		name   string
		answer applyAnswer
		cause  error
	}{
		{string(pkgerrors.CodexTurnEmptyOutput),
			applyAnswer{err: pkgerrors.ErrProviderProducedNoOutput("codex")}, pkgerrors.ErrProviderNoOutput},
		{"whitespace output",
			applyAnswer{response: " \n\t", err: pkgerrors.ErrProviderProducedNoOutput("codex")}, pkgerrors.ErrProviderNoOutput},
		{"nonzero exit",
			applyAnswer{response: "Changed files.", err: pkgerrors.ErrProviderExecutionFailed("codex", executionFailure)},
			executionFailure},
		{"execution error",
			applyAnswer{err: pkgerrors.ErrProviderExecutionFailed("codex", executionFailure)}, executionFailure},
		{"refused CLI",
			applyAnswer{err: pkgerrors.ErrProviderNotRegisteredForCLI("codex")}, pkgerrors.ErrProviderNotRegistered},
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
	for _, command := range []string{
		buildCodeCommand, buildTestsCommand, usRefineCommand, usApplyCommand, scenCheckCommand,
	} {
		t.Run(command, func(t *testing.T) {
			for _, response := range []string{blockAnswer, "no-applied"} {
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
	writerCLI    provider.CLI
	claudeWriter *bool
	codexWriter  *bool
	request      *string
	failRole     provider.Role
	response     string
	err          error
}

func (answer applyAnswer) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, options ...provider.TurnOptions) (string, error) {
	answer.observeApply(role, user, options)

	if role == answer.failRole && answer.err != nil {
		return "", answer.err
	}

	if role == provider.RoleFix {
		return `{"fix_prompt":"Repair the service","questions":[]}`, nil
	}

	if answer.response == blockAnswer {
		return "=== FILE_START: " + user + " ===\napplied: true\nsummary: terms: repaired\n" +
			"=== FILE_END: " + user + " ===", nil
	}

	if answer.response == "no-applied" {
		return "=== FILE_START: " + user + " ===\nsummary: repaired\n=== FILE_END: " + user + " ===", nil
	}

	return answer.response, answer.err
}

type applyUI struct{}

func (applyUI) AskQuestions(_ []engine.ClarifyQuestion) (map[string]string, error) { return map[string]string{}, nil }
func (applyUI) AskApplyRefineOrExit() (engine.UserAction, error) { return engine.UserActionApply, nil }
func (applyUI) AskRefinementFeedback() (string, error) { return "", nil }
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
    coder: codex:test
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

	writerCLI := answer.writerCLI
	if writerCLI == "" {
		writerCLI = provider.CLICodex
	}

	models, err := provider.NewRegistry(
		map[string]string{confirmationTier: "claude:test", "coder": string(writerCLI) + ":test"}, defaults,
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
		RecoverWriterFailures: command == buildCodeCommand,
		LoadItems:             func(context.Context) ([]bool, error) { return []bool{false}, nil },
		GetSubject:            func(bool) (string, string) { return "failing-set", "Failing set" },
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

func TestStartedCodexFailuresRecoverOnlyBuildCodeApply(t *testing.T) {
	for _, command := range []string{
		buildCodeCommand, buildTestsCommand, usRefineCommand, usApplyCommand, scenCheckCommand,
	} {
		for _, role := range provider.Roles() {
			for _, outcome := range []pkgerrors.CodexTurnOutcome{
				pkgerrors.CodexTurnNonZeroExit, pkgerrors.CodexTurnEmptyOutput,
			} {
				t.Run(command+"/"+string(role)+"/"+string(outcome), func(t *testing.T) {
					failure := &pkgerrors.CodexTurnError{Outcome: outcome, Cause: errors.New("writer ended")}
					answer := applyAnswer{err: failure, failRole: role}
					spec := confirmationSpec(t, command, answer)

					useFailureRole(t, role)

					reruns := 0
					spec.PostNotApplied = func(context.Context, bool) (bool, error) {
						reruns++

						return true, nil
					}

					err := runner.Run(t.Context(), spec)
					if command == buildCodeCommand && role == provider.RoleApply {
						if err != nil || reruns != 1 {
							t.Fatalf("Run = %v, reruns = %d", err, reruns)
						}
					} else if !errors.Is(err, failure) || reruns != 0 {
						t.Fatalf("Run = %v, reruns = %d; want fatal", err, reruns)
					}
				})
			}
		}
	}
}

func TestFailedWriterTerminalRerunCannotBypassBound(t *testing.T) {
	failure := &pkgerrors.CodexTurnError{Outcome: pkgerrors.CodexTurnNonZeroExit, Cause: errors.New("writer ended")}
	spec := confirmationSpec(t, buildCodeCommand, applyAnswer{err: failure})
	reruns := 0
	spec.PostNotApplied = func(context.Context, bool) (bool, error) {
		reruns++

		return reruns == 3, nil
	}

	err := runner.Run(t.Context(), spec)
	if err == nil || !strings.Contains(err.Error(), "gave up after 3 applied fix") || reruns != 3 {
		t.Fatalf("Run = %v, reruns = %d", err, reruns)
	}
}

func useFailureRole(t *testing.T, role provider.Role) {
	t.Helper()

	if role != provider.RolePrompt {
		return
	}

	path := "checklists/test.yaml"

	body, err := disk.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	writeConfirmationFile(t, path, strings.Replace(string(body), "D: tests-pass", "Q: Does the test pass?", 1))
}

func TestStartedClaudeFailuresRecoverOnlyBuildCodeApply(t *testing.T) {
	for _, command := range []string{buildCodeCommand, buildTestsCommand, usRefineCommand, usApplyCommand,
		scenCheckCommand} {
		for _, role := range provider.Roles() {
			for _, outcome := range []pkgerrors.ClaudeTurnOutcome{pkgerrors.ClaudeTurnNonZeroExit,
				pkgerrors.ClaudeTurnErrorResult, pkgerrors.ClaudeTurnEmptyOutput} {
				t.Run(command+"/"+string(role)+"/"+string(outcome), func(t *testing.T) {
					failure := &pkgerrors.ClaudeTurnError{Outcome: outcome, Cause: errors.New("rate_limit 429")}
					spec := confirmationSpec(t, command, applyAnswer{err: failure, failRole: role})
					useFailureRole(t, role)

					reruns := 0
					spec.PostNotApplied = func(context.Context, bool) (bool, error) {
						reruns++

						return true, nil
					}

					err := runner.Run(t.Context(), spec)
					if command == buildCodeCommand && role == provider.RoleApply {
						if err != nil || reruns != 1 {
							t.Fatalf("Run=%v reruns=%d", err, reruns)
						}
					} else if !errors.Is(err, failure) || reruns != 0 {
						t.Fatalf("Run=%v reruns=%d; want fatal", err, reruns)
					}
				})
			}
		}
	}
}

func (answer applyAnswer) observeApply(role provider.Role, user string, options []provider.TurnOptions) {
	if role == provider.RoleApply {
		if answer.request != nil {
			*answer.request = user
		}

		if answer.codexWriter != nil {
			*answer.codexWriter = len(options) > 0 && options[0].CodexWriter
		}

		if answer.claudeWriter != nil {
			*answer.claudeWriter = len(options) > 0 && options[0].ClaudeWriter
		}
	}
}

func TestOnlyBuildCodeClaudeApplierRequestsWriter(t *testing.T) {
	for _, command := range []string{buildCodeCommand, buildTestsCommand, usRefineCommand,
		usApplyCommand, scenCheckCommand} {
		t.Run(command, func(t *testing.T) {
			requested := false
			spec := confirmationSpec(t, command, applyAnswer{response: blockAnswer, writerCLI: provider.CLIClaude,
				claudeWriter: &requested})

			err := runner.Run(t.Context(), spec)
			if err != nil {
				t.Fatal(err)
			}

			if requested != (command == buildCodeCommand) {
				t.Fatalf("Claude writer requested=%v for %s", requested, command)
			}
		})
	}
}

func TestOnlyBuildCodeCodexApplierRequestsWriter(t *testing.T) {
	for _, command := range []string{buildCodeCommand, buildTestsCommand, usRefineCommand,
		usApplyCommand, scenCheckCommand} {
		t.Run(command, func(t *testing.T) {
			requested := false
			spec := confirmationSpec(t, command, applyAnswer{response: blockAnswer, writerCLI: provider.CLICodex,
				codexWriter: &requested})

			err := runner.Run(t.Context(), spec)
			if err != nil {
				t.Fatal(err)
			}

			if requested != (command == buildCodeCommand) {
				t.Fatalf("Codex writer requested=%v for %s", requested, command)
			}
		})
	}
}

func TestWriterRequestUsesProviderCommandForms(t *testing.T) {
	body, err := disk.Read("../../../../../templates/build-code.fix-applier.prompt.tpl")
	if err != nil {
		t.Fatal(err)
	}

	_, commands, found := strings.Cut(string(body), "## Test commands")
	if !found {
		t.Fatal("shipped template lost test commands")
	}

	commands = "{{ if .TestCommand }}## Test commands" + strings.Split(commands, "## How to work")[0]

	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		t.Run(string(cli), func(t *testing.T) {
			request := ""
			spec := confirmationSpec(t, buildCodeCommand, applyAnswer{response: "changed", writerCLI: cli, request: &request})
			writeConfirmationFile(t, "user.tpl", commands)
			spec.FixApplier.UseTestCommand(&provider.TestCommand{Command: "go test ./tests/", Selector: "-run", Dir: "/host"})

			_, applyErr := spec.FixApplier.Apply(t.Context(), validate.ApplyParams{SubjectID: "failing-set",
				TmpDir: spec.TmpDir, FixPrompt: "repair", ClaudeWriter: true, CodexWriter: true})
			if !errors.Is(applyErr, pkgerrors.ErrFixApplierNoContent) {
				t.Fatalf("Apply=%v", applyErr)
			}

			for _, form := range []string{"go test ./tests/", "go test ./tests/ -run '<selection>'", "Run in `/host`"} {
				if !strings.Contains(request, form) {
					t.Errorf("missing %s: %s", form, request)
				}
			}

			if cli == provider.CLICodex && !strings.Contains(request, "poll that process with your native process tool") {
				t.Errorf("codex polling instruction missing: %s", request)
			}

			assertClaudeCommandForms(t, cli, request)
		})
	}
}

func assertClaudeCommandForms(t *testing.T, cli provider.CLI, request string) {
	t.Helper()

	if cli != provider.CLIClaude {
		return
	}

	for _, form := range []string{"`cd '/host' && go test ./tests/`",
		"`cd '/host' && go test ./tests/ -run '<selection>'`"} {
		if !strings.Contains(request, form) {
			t.Errorf("missing runnable Claude command %s: %s", form, request)
		}
	}
}
