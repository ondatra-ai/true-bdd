package runner_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

func TestFailedCheckReachesEveryFixTurn(t *testing.T) {
	for _, command := range []string{usRefineCommand, usApplyCommand, scenCheckCommand, "build steps", buildCodeCommand} {
		for _, kind := range []string{handoverCode, "ordinary", handoverSketch, handoverPlainSketch} {
			for _, concurrency := range []int{1, 2} {
				t.Run(command+"/"+kind+"/"+strconv.Itoa(concurrency), func(t *testing.T) {
					reply := &handoverReply{t: t}
					spec := handoverSpec(t, reply, command, kind)

					spec.ValidationConcurrency = concurrency

					err := runner.Run(t.Context(), spec)
					if err != nil {
						t.Fatal(err)
					}

					if len(reply.proposals) != 2 || len(reply.applies) != 1 {
						t.Fatalf("proposals=%d applies=%d", len(reply.proposals), len(reply.applies))
					}

					assertHandover(t, reply, kind)
				})
			}
		}
	}
}

type handoverReply struct {
	t         *testing.T
	prompts   int
	question  string
	proposals []handoverProposal
	applies   []handoverApply
}

func (reply *handoverReply) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions,
) (string, error) {
	switch role {
	case provider.RolePrompt:
		reply.prompts++
		if reply.prompts == 1 {
			reply.question = user

			return `{"answer":"fail","context":["missing body","needs repair"],"fix_prompt":"Repair only this finding"}`, nil
		}

		return `{"answer":"pass","context":[],"fix_prompt":""}`, nil
	case provider.RoleFix:
		var data handoverProposal

		err := json.Unmarshal([]byte(user), &data)
		if err != nil {
			reply.t.Fatal(err)
		}

		reply.proposals = append(reply.proposals, data)
		if len(reply.proposals) == 1 {
			return `{"fix_prompt":"","questions":[{"id":"q1","question":"Which body?",` +
				`"context":"missing body","options":["this body"]}]}`, nil
		}

		return `{"fix_prompt":"Approved body repair","questions":[]}`, nil
	case provider.RoleApply:
		var data handoverApply

		err := json.Unmarshal([]byte(user), &data)
		if err != nil {
			reply.t.Fatal(err)
		}

		reply.applies = append(reply.applies, data)

		return outcomeBlock(data.ResultPath, "applied: true"), nil
	}

	return "", nil
}

func handoverSpec(t *testing.T, reply *handoverReply, command, kind string) runner.Spec[bool] {
	t.Helper()
	spec := confirmationSpec(t, command, applyAnswer{})
	spec.ChecklistName = "handover"
	question := "Q: {prompt: Is the body written}"
	with := ""

	if kind == handoverCode {
		question = "Q: {code: tests-pass}"
	}

	if kind == handoverSketch || kind == handoverPlainSketch {
		with = "\n    with: {concepts: sketch}"
	}

	for path, body := range map[string]string{
		"checklists/handover.yaml": "sections:\n- id: verdict\n  validation_prompts:\n  - " + question + with +
			"\n    rationale: A body must exist\n    docs: [product, architecture_yaml]\n" +
			"    F: Repair only this finding\n    fix_model: high\n    apply_model: coder\n",
		"docs/product/product.yaml": "concepts:\n  when:\n  - concept: halving\n" +
			"    means: The Owner halves a whole number.\n    terms: []\n",
		"docs/architecture/architecture.yaml": "architecture:\n  services: []\n",
		"question.tpl":                        "{{ .Question }}",
		"proposal.tpl": `{{ dict "failed_check" .FailedCheck "subject_id" .SubjectID "iteration" .Iteration ` +
			`"doc_paths" .DocPaths "user_answers" .UserAnswers | toJson }}`,
		"apply.tpl": `{{ dict "judgement" .Judgement "check" .Check "fix_prompt" .FixPrompt ` +
			`"subject_id" .SubjectID "result_path" .ResultPath | toJson }}`,
	} {
		writeConfirmationFile(t, path, body)
	}

	if kind == handoverPlainSketch {
		writeConfirmationFile(t, "docs/product/product.yaml", "roles: []\n")
	}

	spec.UI = handoverUI{}

	body, err := disk.Read("true-bdd/true-bdd.yaml")
	if err != nil {
		t.Fatal(err)
	}

	writeConfirmationFile(t, "true-bdd/true-bdd.yaml", string(body)+
		"\ndocuments:\n  product: docs/product/product.yaml\n  architecture_yaml: docs/architecture/architecture.yaml\n")

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{confirmationTier: "claude:judge", handoverCoder: "codex:writer"},
		map[provider.Role]string{provider.RolePrompt: confirmationTier, provider.RoleFix: confirmationTier,
			provider.RoleApply: handoverCoder})
	if err != nil {
		t.Fatal(err)
	}

	spec.Checks = checks.Set[bool]{handoverCheck: {Run: func(passed bool, _ checks.Input) (checks.Verdict, error) {
		return checks.Verdict{Pass: passed, Context: []string{handoverFinding, handoverDetail}}, nil
	}}}
	spec.DocResolver = docs.NewResolver(cfg)
	spec.Evaluator = validate.NewChecklistEvaluatorWithPaths(reply, cfg, models, "system.tpl", "question.tpl")
	spec.FixGenerator = validate.NewFixPromptGeneratorWithPaths(reply, cfg, models, "system.tpl", "proposal.tpl")
	spec.FixApplier = validate.NewFixApplierWithPaths(reply, cfg, models, "system.tpl", "apply.tpl")

	return spec
}

func assertHandover(t *testing.T, reply *handoverReply, kind string) {
	t.Helper()

	want := map[string]any{
		"SectionPath": "handover/verdict", "Question": handoverQuestion, "ActualAnswer": "fail",
		"Context": []any{handoverFinding, handoverDetail}, "Status": "FAIL", "Rationale": handoverRationale,
		"FixPrompt": handoverRepair, "PromptIndex": float64(1), "Docs": []any{docs.KeyProduct, docs.KeyArchitectureYAML},
		"FixModelTier": confirmationTier, "ApplyModelTier": handoverCoder, "Judgement": "", "Check": "",
	}
	if kind == handoverCode {
		want["Question"], want["Check"] = "Q code: tests-pass", handoverCheck
	}

	if kind == handoverSketch {
		want["Judgement"] = handoverSketch
	}

	for index, data := range reply.proposals {
		got := data.FailedCheck
		if kind == handoverSketch {
			question, _ := got["Question"].(string)
			if !strings.Contains(question, "For every AC, draft a runnable test sketch") {
				t.Errorf("lost effective sketch question: %q", question)
			}

			want["Question"] = reply.question
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("turn %d FailedCheck = %#v; want %#v", index+1, got, want)
		}

		assertHandoverMetadata(t, data, index+1)
	}

	assertApplyHandover(t, reply.applies[0], want)
}

func assertApplyHandover(t *testing.T, applied handoverApply, want map[string]any) {
	t.Helper()

	got := []string{applied.Judgement, applied.Check, applied.FixPrompt, applied.SubjectID}

	expected := []any{want["Judgement"], want["Check"], handoverApproved, handoverID}
	for index, value := range got {
		if value != expected[index] {
			t.Errorf("apply handover lost: %#v", applied)
		}
	}
}

type handoverProposal struct {
	FailedCheck map[string]any    `json:"failed_check"`
	SubjectID   string            `json:"subject_id"`
	Iteration   int               `json:"iteration"`
	DocPaths    map[string]string `json:"doc_paths"`
	UserAnswers map[string]string `json:"user_answers"`
}

type handoverApply struct {
	Judgement  string `json:"judgement"`
	Check      string `json:"check"`
	FixPrompt  string `json:"fix_prompt"`
	SubjectID  string `json:"subject_id"`
	ResultPath string `json:"result_path"`
}

const (
	handoverCode      = "code"
	handoverSketch    = "sketch"
	handoverFinding   = "missing body"
	handoverDetail    = "needs repair"
	handoverRepair    = "Repair only this finding"
	handoverApproved  = "Approved body repair"
	handoverRationale = "A body must exist"
	handoverQuestion  = "Is the body written"
	handoverCheck     = "tests-pass"
	handoverCoder     = "coder"
	handoverID        = "failing-set"
)

type handoverUI struct{ applyUI }

func (handoverUI) AskQuestions(_ []engine.ClarifyQuestion) (map[string]string, error) {
	return map[string]string{"q1": "this body"}, nil
}

func assertHandoverMetadata(t *testing.T, data handoverProposal, iteration int) {
	t.Helper()

	if data.Iteration != iteration || data.SubjectID != handoverID {
		t.Errorf("proposal metadata lost: %#v", data)
	}

	paths := map[string]string{
		"product":           "docs/product/product.yaml",
		"architecture_yaml": "docs/architecture/architecture.yaml",
	}
	if !reflect.DeepEqual(data.DocPaths, paths) {
		t.Errorf("proposal document paths lost: %#v", data.DocPaths)
	}

	answers := map[string]string{}
	if iteration > 1 {
		answers["q1"] = "this body"
	}

	if !reflect.DeepEqual(data.UserAnswers, answers) {
		t.Errorf("clarification answer lost: %#v", data.UserAnswers)
	}
}

const handoverPlainSketch = "sketch without concepts"
