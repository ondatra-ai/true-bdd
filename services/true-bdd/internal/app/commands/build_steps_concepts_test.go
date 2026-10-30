package commands_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
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
	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	deps := buildFixture(t)

	fixture := filepath.Join(repository, "tests/true-bdd/fixtures/concept-grouping-batches/input")

	err = disk.CopyTree(".", fixture, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	duplicateGroupingScenario(t)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models, err := provider.NewRegistry(map[string]string{groupTestTier: "claude:test"}, map[provider.Role]string{
		provider.RolePrompt: groupTestTier, provider.RoleFix: groupTestTier, provider.RoleApply: groupTestTier,
	})
	if err != nil {
		t.Fatal(err)
	}

	reply := &groupingReplies{}
	deps.Evaluator = validate.NewChecklistEvaluator(reply, cfg, models)

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if !errors.Is(err, commands.ErrPatternProposalPending) {
		t.Fatal(err)
	}

	if !slices.Equal(reply.sizes, []int{3, 2, 5}) {
		t.Fatalf("group batches then binding context: %v", reply.sizes)
	}
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
