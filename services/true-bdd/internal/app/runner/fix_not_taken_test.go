package runner_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

const refusedKey = "then concept 'result' declares variations"

// notTakenAnswer records every fix turn's request and confirms every apply.
type notTakenAnswer struct {
	fixRequests *[]string
}

func (answer notTakenAnswer) ExecutePromptWithSystem(_ context.Context, role provider.Role, _, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions) (string, error) {
	if role == provider.RoleFix {
		*answer.fixRequests = append(*answer.fixRequests, user)

		return `{"fix_prompt":"Rewrite the means","questions":[]}`, nil
	}

	return "=== FILE_START: " + user + " ===\napplied: true\n=== FILE_END: " + user + " ===", nil
}

// A fix PostFix does not take keeps the walk going: the Prompt is asked
// again and the refusal reaches the next fix turn as a finding (#268).
func TestAFixNotTakenReturnsItsRefusalToTheNextFix(t *testing.T) {
	requests := make([]string, 0)
	spec := notTakenSpec(t, notTakenAnswer{fixRequests: &requests})
	posts := 0
	spec.PostFix = func(_ context.Context, item bool, _ string) (bool, error) {
		posts++
		if posts == 1 {
			return item, fmt.Errorf("product: %w: %s", runner.ErrFixNotTaken, refusedKey)
		}

		return true, nil
	}

	err := runner.Run(t.Context(), spec)
	if err != nil {
		t.Fatalf("Run = %v, want the second fix taken", err)
	}

	if len(requests) != 2 || strings.Contains(requests[0], refusedKey) || !strings.Contains(requests[1], refusedKey) {
		t.Fatalf("fix requests = %q, want the refusal in the second only", requests)
	}

	if !strings.Contains(requests[1], "still failing") {
		t.Errorf("second fix request = %q, want the check's own finding kept", requests[1])
	}
}

// A fix never taken is bounded like any fix that does not converge.
func TestAFixNeverTakenStopsAtTheBound(t *testing.T) {
	requests := make([]string, 0)
	spec := notTakenSpec(t, notTakenAnswer{fixRequests: &requests})
	spec.PostFix = func(_ context.Context, item bool, _ string) (bool, error) {
		return item, fmt.Errorf("product: %w: %s", runner.ErrFixNotTaken, refusedKey)
	}

	err := runner.Run(t.Context(), spec)
	if err == nil || !strings.Contains(err.Error(), "gave up after 2 applied fix") || len(requests) != 2 {
		t.Fatalf("Run = %v after %d fixes, want the bound after two", err, len(requests))
	}
}

func notTakenSpec(t *testing.T, answer ports.AIPort) runner.Spec[bool] {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)

	for path, body := range map[string]string{
		"true-bdd/true-bdd.yaml": "paths:\n  checklists_dir: checklists\nengine:\n  models:\n    high: claude:test\n" +
			"  default_prompt_model: high\n  default_fix_model: high\n  default_apply_model: high\n",
		"checklists/refusal.yaml": "version: '1.0'\nconfig:\n  max_apply_attempts: 2\nsections:\n- id: verdict\n" +
			"  validation_prompts:\n  - Q: {code: tests-pass}\n    F: Repair the product\n",
		"system.tpl": "Fix it.",
		"fix.tpl":    "{{ range .FailedCheck.Context }}{{ . }}\n{{ end }}",
		"apply.tpl":  "{{.ResultPath}}",
	} {
		writeConfirmationFile(t, path, body)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{confirmationTier: "claude:test"}, map[provider.Role]string{
		provider.RolePrompt: confirmationTier, provider.RoleFix: confirmationTier, provider.RoleApply: confirmationTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(root, "tmp")

	err = disk.Dir(tmp, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	return runner.Spec[bool]{
		Name: usRefineCommand, ChecklistName: "refusal", Fix: true, TmpDir: tmp,
		LoadItems:  func(context.Context) ([]bool, error) { return []bool{false}, nil },
		GetSubject: func(bool) (string, string) { return "host", "Host" },
		Checks: checks.Set[bool]{"tests-pass": {Run: func(passed bool, _ checks.Input) (checks.Verdict, error) {
			return checks.Verdict{Pass: passed, Context: []string{"still failing"}}, nil
		}}},
		Evaluator:       validate.NewChecklistEvaluatorWithPaths(answer, cfg, models, "system.tpl", "apply.tpl"),
		FixGenerator:    validate.NewFixPromptGeneratorWithPaths(answer, cfg, models, "system.tpl", "fix.tpl"),
		FixApplier:      validate.NewFixApplierWithPaths(answer, cfg, models, "system.tpl", "apply.tpl"),
		ChecklistLoader: checklist.NewChecklistLoader(cfg), DocResolver: docs.NewResolver(cfg),
		Renderer: runner.NewTableRenderer(), UI: applyUI{},
		Finalize: func(*engine.Result[bool]) error { return nil },
	}
}
