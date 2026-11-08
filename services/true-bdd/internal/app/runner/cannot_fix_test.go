package runner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/logging"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

const proposalStage = "proposal"
const applyStage = "apply"
const clarificationStage = "clarification"
const cannotFixServicePath = "services/calc"

const cannotFixJSON = `{"reason":"missing operation","correction":"extend the interface"}`

func TestCannotFixStopsAtProposalApplyAndClarification(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		for _, stage := range []string{proposalStage, applyStage, clarificationStage} {
			t.Run(string(cli)+"/"+stage, func(t *testing.T) {
				reply := &outcomeReply{stage: stage, outcome: `{"cannot_fix":` + cannotFixJSON + `}`}
				spec := outcomeSpec(t, reply, cli)
				err := runner.Run(t.Context(), spec)
				assertCannotFix(t, err, "missing operation", "extend the interface")

				wantFixes, wantApplies := 1, 0
				if stage == clarificationStage {
					wantFixes = 2
				}

				if stage == applyStage {
					wantApplies = 1
				}

				if reply.fixes != wantFixes || reply.applies != wantApplies {
					t.Fatalf("fix turns=%d apply turns=%d", reply.fixes, reply.applies)
				}
			})
		}
	}
}

func TestExistingCannotFixInstancesKeepTheirStopAndRerun(t *testing.T) {
	for _, test := range []struct {
		name, stage, outcome, reason, correction string
		rerun                                    bool
	}{
		{"interface", applyStage, `{"interface_cannot_serve":{"concept":"halving","lacks":"no halve operation"}}`,
			"no halve operation", "extend the Public interface declaration", false},
		{"impossible plan", proposalStage, `{"impossible":[{"path":"config.yaml","reason":"required undeclared setting"}]}`,
			missingSettingReason, manualFileCorrection, true},
		{"impossible apply", applyStage, `{"impossible":[{"path":"config.yaml","reason":"required undeclared setting"}]}`,
			missingSettingReason, manualFileCorrection, true},
		{"partly applied impossible repair", applyStage, `{"applied":true,"summary":"service repaired",` +
			`"impossible":[{"path":"config.yaml","reason":"required undeclared setting"}]}`,
			missingSettingReason, manualFileCorrection, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			reply := &outcomeReply{stage: test.stage, outcome: test.outcome}
			spec := outcomeSpec(t, reply, provider.CLIClaude)
			spec.FixGenerator.UseImpossibleReport([]string{cannotFixServicePath})

			reruns := 0
			spec.StopImpossible = func(_ context.Context, _ bool, obstruction error) error {
				reruns++

				return obstruction
			}
			err := runner.Run(t.Context(), spec)
			assertCannotFix(t, err, test.reason, test.correction)

			if (reruns == 1) != test.rerun {
				t.Fatalf("reruns=%d", reruns)
			}
		})
	}
}

func assertCannotFix(t *testing.T, err error, details ...string) {
	t.Helper()

	if err == nil {
		t.Fatal("cannot-fix outcome did not stop the run")
	}

	for _, want := range append([]string{"cannot fix checklist item", "test/verdict #1"}, details...) {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %v omits %q", err, want)
		}
	}
}

type outcomeReply struct {
	stage, outcome string
	schema         string
	fixes, applies int
}

func (reply *outcomeReply) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	if role == provider.RoleApply {
		reply.applies++

		return outcomeBlock(user, reply.outcome), nil
	}

	reply.schema = schema

	reply.fixes++
	if reply.stage == clarificationStage && reply.fixes == 1 {
		questions := `{"questions":[{"id":"q1","question":"Which interface?",` +
			`"context":"Missing operation","options":["calc"]}],"fix_prompt":""}`
		if schema == "" {
			return "=== QUESTIONS_START ===\n" + questions + "\n=== QUESTIONS_END ===", nil
		}

		return questions, nil
	}

	response := reply.outcome
	if reply.stage == applyStage {
		response = `{"fix_prompt":"repair","questions":[]}`
	}

	if schema == "" {
		return outcomeBlock(user, response), nil
	}

	return response, nil
}

func outcomeBlock(path, content string) string {
	return "=== FILE_START: " + path + " ===\n" + content + "\n=== FILE_END: " + path + " ==="
}

func outcomeSpec(t *testing.T, reply *outcomeReply, cli provider.CLI) runner.Spec[bool] {
	t.Helper()
	spec := confirmationSpec(t, usRefineCommand, applyAnswer{})

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{confirmationTier: string(cli) + ":test"},
		map[provider.Role]string{provider.RolePrompt: confirmationTier,
			provider.RoleFix: confirmationTier, provider.RoleApply: confirmationTier})
	if err != nil {
		t.Fatal(err)
	}

	spec.FixGenerator = validate.NewFixPromptGeneratorWithPaths(reply, cfg, models, "system.tpl", "user.tpl")
	spec.FixApplier = validate.NewFixApplierWithPaths(reply, cfg, models, "system.tpl", "user.tpl")

	return spec
}

// Codex requires every schema property, so a fixable F must be able to answer null.
func TestStructuredFixProposalAcceptsANullCannotFixOutcome(t *testing.T) {
	reply := &outcomeReply{stage: applyStage, outcome: `{"applied":true}`}
	spec := outcomeSpec(t, reply, provider.CLICodex)

	err := runner.Run(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}

	var resource any

	err = json.Unmarshal([]byte(reply.schema), &resource)
	if err != nil {
		t.Fatal(err)
	}

	compiler := jsonschema.NewCompiler()

	err = compiler.AddResource("fix.json", resource)
	if err != nil {
		t.Fatal(err)
	}

	schema, err := compiler.Compile("fix.json")
	if err != nil {
		t.Fatal(err)
	}

	err = schema.Validate(map[string]any{"fix_prompt": "repair", "questions": []any{}, "cannot_fix": nil})
	if err != nil {
		t.Fatalf("fixable F cannot satisfy the native strict schema: %v", err)
	}
}

func TestMalformedCannotFixAnswersStopWithoutApplying(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		for _, stage := range []string{proposalStage, applyStage, clarificationStage} {
			for _, test := range []struct{ name, outcome string }{
				{"blank reason", `{"cannot_fix":{"reason":"  ","correction":"extend interface"}}`},
				{"blank correction", `{"cannot_fix":{"reason":"missing operation","correction":"\t"}}`},
				{"missing correction", `{"cannot_fix":{"reason":"missing operation"}}`},
				{"mixed plan", `{"fix_prompt":"repair", "cannot_fix":` + cannotFixJSON + `}`},
				{"mixed questions", `{"questions":[{"id":"q1"}], "cannot_fix":` + cannotFixJSON + `}`},
				{"mixed apply", `{"applied":true, "cannot_fix":` + cannotFixJSON + `}`},
				{"mixed document", `{"story":{"title":"repaired story"},"cannot_fix":` + cannotFixJSON + `}`},
				{"impossible with plan", `{"fix_prompt":"repair","impossible":[{"path":"config.yaml"}],` +
					`"cannot_fix":` + cannotFixJSON + `}`},
				{"impossible with questions", `{"questions":[{"id":"q1"}],"impossible":[{"path":"config.yaml"}],` +
					`"cannot_fix":` + cannotFixJSON + `}`},
				{"impossible with apply", `{"applied":true,"impossible":[{"path":"config.yaml"}],` +
					`"cannot_fix":` + cannotFixJSON + `}`},
			} {
				t.Run(string(cli)+"/"+stage+"/"+test.name, func(t *testing.T) {
					reply := &outcomeReply{stage: stage, outcome: test.outcome}

					spec := outcomeSpec(t, reply, cli)
					spec.FixGenerator.UseImpossibleReport([]string{cannotFixServicePath})

					err := runner.Run(t.Context(), spec)
					if !errors.Is(err, pkgerrors.ErrCannotFixMalformed) || errors.Is(err, pkgerrors.ErrCannotFix) {
						t.Fatalf("malformed answer must be distinct from cannot-fix: %v", err)
					}

					if reply.applies > 0 && stage != applyStage {
						t.Fatal("malformed proposal reached apply")
					}
				})
			}
		}
	}
}

func TestCannotFixAllowsReportsAtEveryStage(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		for _, stage := range []string{proposalStage, clarificationStage, applyStage} {
			for _, report := range []string{
				`"summary":"investigated the permitted solution"`,
				`"target":"services/calc"`, `"applied":false`, `"impossible":[]`,
				`"summary":"no permitted solution","target":"services/calc","applied":false,"impossible":[]`,
			} {
				t.Run(string(cli)+"/"+stage+"/"+report, func(t *testing.T) {
					reply := &outcomeReply{stage: stage, outcome: `{` + report + `,"cannot_fix":` + cannotFixJSON + `}`}

					err := runner.Run(t.Context(), outcomeSpec(t, reply, cli))
					if !errors.Is(err, pkgerrors.ErrCannotFix) || errors.Is(err, pkgerrors.ErrCannotFixMalformed) {
						t.Fatalf("report must preserve the cannot-fix outcome: %v", err)
					}

					assertCannotFix(t, err, "missing operation", "extend the interface")
				})
			}
		}
	}
}

func TestImpossibleWithCannotFixKeepsExistingPathAtEveryStage(t *testing.T) {
	for _, cli := range []provider.CLI{provider.CLIClaude, provider.CLICodex} {
		for _, stage := range []string{proposalStage, clarificationStage, applyStage} {
			t.Run(string(cli)+"/"+stage, func(t *testing.T) {
				assertCombinedImpossible(t, cli, stage)
			})
		}
	}
}

func assertCombinedImpossible(t *testing.T, cli provider.CLI, stage string) {
	t.Helper()

	reply := &outcomeReply{stage: stage, outcome: `{"fix_prompt":"","questions":[],"applied":false,` +
		`"summary":"no permitted solution","impossible":[{"path":"config.yaml","reason":"undeclared setting"}],` +
		`"cannot_fix":` + cannotFixJSON + `}`}
	spec := outcomeSpec(t, reply, cli)
	spec.FixGenerator.UseImpossibleReport([]string{cannotFixServicePath})

	var captured bytes.Buffer

	original := slog.Default()

	slog.SetDefault(slog.New(logging.Handler(&captured, "")))
	t.Cleanup(func() { slog.SetDefault(original) })

	reruns := 0
	spec.StopImpossible = func(_ context.Context, _ bool, obstruction error) error {
		reruns++

		if !errors.Is(obstruction, pkgerrors.ErrImpossibleWork) {
			t.Errorf("existing impossible path lost: %v", obstruction)
		}

		return obstruction
	}
	err := runner.Run(t.Context(), spec)
	assertCannotFix(t, err, "missing operation", "extend the interface", "config.yaml: undeclared setting",
		"change these files by hand, or add them to the service's writable")

	if reruns != 1 || !errors.Is(err, pkgerrors.ErrImpossibleWork) || errors.Is(err, pkgerrors.ErrCannotFixMalformed) {
		t.Fatalf("combined outcome must rerun once on existing impossible path: reruns=%d, error=%v", reruns, err)
	}

	for _, detail := range []string{
		"Walk stopped", "config.yaml: undeclared setting", "missing operation", "extend the interface",
	} {
		if !strings.Contains(captured.String(), detail) {
			t.Errorf("log omits %q: %s", detail, captured.String())
		}
	}

	if stage == applyStage && !strings.Contains(captured.String(), "Writer reported impossible repair") {
		t.Error("existing writer impossible log missing")
	}
}

const manualFileCorrection = "change these files by hand"

const missingSettingReason = "required undeclared setting"
