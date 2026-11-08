package commands_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
 "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
)

type verificationReplies struct {
	conceptReplies

	category               string
	verifications, applies int
	turns                  []string
}

func (reply *verificationReplies) ExecutePromptWithSystem(
	ctx context.Context, role provider.Role, system, user string, model provider.ModelRef,
	mode provider.ExecutionMode, schema string, options ...provider.TurnOptions,
) (string, error) {
	switch {
	case role == provider.RoleFix:
		reply.turns = append(reply.turns, "fix")

		return `{"fix_prompt":"Write the named bodies.","questions":[],"cannot_fix":null}`, nil
	case role == provider.RoleApply:
		reply.turns = append(reply.turns, "apply")
		reply.applies++

		return writeVerificationBodies(user)
	case strings.Contains(schema, `"findings"`):
		reply.turns = append(reply.turns, "verify")

		reply.verifications++
		if reply.verifications == 1 && reply.category != "" {
			definitions, err := stepskeleton.Concepts("tests/calc/steps")
			if err != nil {
				return "", err
			}

			return fmt.Sprintf(`{"answer":"fail","findings":[{"definition":%q,"category":%q,`+
				`"finding":"Repair this discrepancy only."}]}`, definitions[0].Function, reply.category), nil
		}

		return `{"answer":"pass","findings":[]}`, nil
	case strings.Contains(schema, `"patterns"`):
		reply.turns = append(reply.turns, "pattern")
	}

	return reply.conceptReplies.ExecutePromptWithSystem(ctx, role, system, user, model, mode, schema, options...)
}

func TestBuildStepsWalkRunsOneVerificationFixAndRechecksItsQuestion(t *testing.T) {
	for _, category := range []string{"concept_binding", patternCaptureCategory} {
		t.Run(category, func(t *testing.T) {
			pattern := `^Press Halve once for (\w+)\.$`

			wrong := `^Press Halve once for (\w)\w*\.$`
			if category == "concept_binding" {
				wrong = pattern
			}

			answer := func(pattern string) string {
				return fmt.Sprintf(`{"patterns":[{"pattern":%q,`+
					`"captures":[{"name":"number","means":"The number supplied."}],"serves":"Press Halve once."}]}`, pattern)
			}
			reply := &verificationReplies{category: category,
				conceptReplies: conceptReplies{proposals: []string{answer(wrong), answer(pattern)}}}
			deps := conceptHost(t, reply)
			prepareVerificationWalk(t, &deps, reply)

			err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
			if err != nil {
				t.Fatal(err)
			}

			want := "pattern,fix,apply,verify,fix,apply,verify,verify,verify"
			if category == patternCaptureCategory {
				want = "pattern,fix,apply,verify,pattern,fix,apply,verify,verify,verify"
			}

			if strings.Join(reply.turns, ",") != want {
				t.Fatalf("turns=%v, want %s", reply.turns, want)
			}
		})
	}
}

func prepareVerificationWalk(t *testing.T, deps *commands.BuildStepsDeps, reply ports.AIPort) {
	t.Helper()

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	deps.UserInputCollector = input.NewUserInputCollectorFrom(strings.NewReader(strings.Repeat("1\n", 12)))

	deps.TableRenderer = runner.NewTableRenderer()
	// Use the shipped build-steps apply pair, rather than the generic Story writer.
	deps.FixApplier = validate.NewFixApplierWithPaths(reply, cfg, testModels(t),
		"templates/build-steps.fix-applier.system.prompt.tpl", "templates/build-steps.fix-applier.prompt.tpl")
}

func writeVerificationBodies(user string) (string, error) {
 return writeVerificationSuiteBodies(user, "tests/calc/steps")
}

func writeVerificationSuiteBodies(user, suite string) (string, error) {
	definitions, err := stepskeleton.Concepts(suite)
	if err != nil {
		return "", err
	}

	for _, definition := range definitions {
		source := fmt.Sprintf("package steps\n//truebdd:concept %s %q\n"+
			"func %s(state *State,args []string) error { return nil }\n",
			definition.Block, definition.Concept, definition.Function)

		err = disk.Write(definition.File, []byte(source), disk.Shared)
		if err != nil {
			return "", err
		}
	}

	result := regexp.MustCompile(`=== FILE_START: ([^\n]+) ===`).FindStringSubmatch(user)
	if len(result) != 2 {
		return "", fmt.Errorf("no apply result path: %s", user)
	}

	return "=== FILE_START: " + result[1] + " ===\napplied: true\nsummary: Wrote bodies.\n" +
		"=== FILE_END: " + result[1] + " ===", nil
}
