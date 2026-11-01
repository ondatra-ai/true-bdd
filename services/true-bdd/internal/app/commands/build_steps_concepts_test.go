package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/ports"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const groupTestTier = "high"

type groupingReplies struct{ sizes []int }

func (reply *groupingReplies) ExecutePromptWithSystem(_ context.Context, _ provider.Role, _ string, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, _ string, _ ...provider.TurnOptions,
) (string, error) {
	var request struct {
		Steps []struct {
			Text string `json:"text"`
		} `json:"steps"`
	}

	err := json.Unmarshal([]byte(user), &request)
	if err != nil {
		return "", fmt.Errorf("read grouping request: %w", err)
	}

	reply.sizes = append(reply.sizes, len(request.Steps))
	switch len(reply.sizes) {
	case 1:
		return `{"assignments":[
 {"index":1,"concept":"halve","score":10,"runner_up":"double","runner_up_score":0},
 {"index":2,"concept":"halve","score":10,"runner_up":"double","runner_up_score":0},
 {"index":3,"concept":"halve","score":10,"runner_up":"double","runner_up_score":0}]}`, nil
	case 2:
		return `{"assignments":[
 {"index":1,"concept":"halve","score":10,"runner_up":"double","runner_up_score":0},
 {"index":2,"concept":"halve","score":10,"runner_up":"double","runner_up_score":0}]}`, nil
	default:
		return `{"bindings":{"when:halve":{"unambiguous":true,"reason":"Press the named button once."}}}`, nil
	}
}

func TestConceptGroupingBalancesDistinctUnboundTextsBeforeBindingJudgement(t *testing.T) {
	reply := &groupingReplies{}
	deps := groupingHost(t, reply)

	duplicateGroupingScenario(t)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	if !slices.Equal(reply.sizes, []int{3, 2, 5}) {
		t.Fatalf("group batches then binding context: %v", reply.sizes)
	}
}

// bindingTurns groups each batch's first step under double and the rest under
// halve, and records every binding turn's keys and step texts.
type bindingTurns struct {
	turns []bindingTurn
}

type bindingTurn struct {
	keys  []string
	steps []string
}

func (reply *bindingTurns) ExecutePromptWithSystem(_ context.Context, _ provider.Role, _ string, user string,
	_ provider.ModelRef, _ provider.ExecutionMode, schema string, _ ...provider.TurnOptions,
) (string, error) {
	var request struct {
		Steps []struct {
			Text string `json:"text"`
		} `json:"steps"`
		Bindings map[string]json.RawMessage `json:"bindings"`
	}

	err := json.Unmarshal([]byte(user), &request)
	if err != nil {
		return "", fmt.Errorf("read build steps request: %w", err)
	}

	if !strings.Contains(schema, `"bindings"`) {
		assignments := make([]string, 0, len(request.Steps))
		for index := range request.Steps {
			concept, runnerUp := "halve", "double"
			if index == 0 {
				concept, runnerUp = runnerUp, concept
			}

			assignments = append(assignments, fmt.Sprintf(
				`{"index":%d,"concept":%q,"score":10,"runner_up":%q,"runner_up_score":0}`, index+1, concept, runnerUp))
		}

		return `{"assignments":[` + strings.Join(assignments, ",") + `]}`, nil
	}

	turn := bindingTurn{keys: slices.Sorted(maps.Keys(request.Bindings))}
	for _, step := range request.Steps {
		turn.steps = append(turn.steps, step.Text)
	}

	reply.turns = append(reply.turns, turn)
	verdicts := make([]string, 0, len(turn.keys))

	for _, key := range turn.keys {
		verdicts = append(verdicts, fmt.Sprintf(`%q:{"unambiguous":true,"reason":"Press the named button once."}`, key))
	}

	return `{"bindings":{` + strings.Join(verdicts, ",") + `}}`, nil
}

// build steps judges each binding as scen check does (ADR-0037): one turn per
// binding, over the steps of the suite that reach it.
func TestBuildStepsJudgesEachBindingInItsOwnTurnOverTheStepsReachingIt(t *testing.T) {
	reply := &bindingTurns{}
	deps := groupingHost(t, reply)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	want := []bindingTurn{
		{keys: []string{"when:double"}, steps: []string{"Press Halve once for four.", "Press Halve once for one."}},
		{keys: []string{"when:halve"}, steps: []string{"Press Halve once for five.", "Press Halve once for three.",
			"Press Halve once for two."}},
	}

	if fmt.Sprint(reply.turns) != fmt.Sprint(want) {
		t.Fatalf("one binding turn per binding with the steps reaching it, want %v, got %v", want, reply.turns)
	}
}

// A missing bindings Prompt is a startup refusal, made before any grouping turn.
func TestBuildStepsRefusesAMissingBindingsPromptBeforeGrouping(t *testing.T) {
	reply := &bindingTurns{}
	deps := groupingHost(t, reply)

	checklistFile := "true-bdd/checklists/scen-check.yaml"

	body, err := disk.Read(checklistFile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "concepts: bindings") {
		t.Fatalf("%s asks no bindings Prompt", checklistFile)
	}

	err = disk.Write(checklistFile, []byte(strings.Replace(string(body), "concepts: bindings", "concepts: words", 1)),
		disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrBuildStepsChecklist) || !strings.Contains(err.Error(), "missing bindings Prompt") {
		t.Fatalf("want the missing bindings Prompt refused, got %v", err)
	}

	if !strings.Contains(output.String(), `msg="Refusing to start"`) || len(reply.turns) != 0 ||
		strings.Contains(output.String(), "Grouping batch") {
		t.Fatalf("a startup refusal before any grouping turn, got:\n%s", output.String())
	}
}

// groupingHost is concept-grouping-batches' host, its evaluator's turns answered by reply.
func groupingHost(t *testing.T, reply ports.AIPort) commands.BuildStepsDeps {
	t.Helper()

	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	deps := buildFixture(t)

	err = disk.CopyTree(".", filepath.Join(repository, "tests/true-bdd/fixtures/concept-grouping-batches/input"),
		disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	deps.Evaluator = validate.NewChecklistEvaluator(reply, cfg, testModels(t))

	return deps
}

// testModels answers every role at one test tier.
func testModels(t *testing.T) *provider.Registry {
	t.Helper()

	models, err := provider.NewRegistry(map[string]string{groupTestTier: "claude:test"}, map[provider.Role]string{
		provider.RolePrompt: groupTestTier, provider.RoleFix: groupTestTier, provider.RoleApply: groupTestTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	return models
}

func duplicateGroupingScenario(t *testing.T) {
	t.Helper()
	// Repeated Scenario texts do not add grouping work.
	body, err := disk.Read("docs/scenarios.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]map[string]any

	err = json.Unmarshal(body, &document)
	if err != nil {
		t.Fatal(err)
	}

	document["scenarios"]["host-002"] = document["scenarios"]["host-001"]

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("docs/scenarios.yaml", encoded, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}
