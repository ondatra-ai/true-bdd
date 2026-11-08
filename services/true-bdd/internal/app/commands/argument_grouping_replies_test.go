package commands_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

type argumentGroupingReplies struct {
	groupingReplies

	arguments []string
}

func (reply *argumentGroupingReplies) ExecutePromptWithSystem(ctx context.Context, role provider.Role,
	system, user string, model provider.ModelRef, mode provider.ExecutionMode, schema string,
	options ...provider.TurnOptions,
) (string, error) {
	var request struct {
		Arguments string `json:"step_arguments"`
	}

	err := json.Unmarshal([]byte(user), &request)
	if err != nil {
		return "", fmt.Errorf("decode argument evidence: %w", err)
	}

	reply.arguments = append(reply.arguments, request.Arguments)

	return reply.groupingReplies.ExecutePromptWithSystem(ctx, role, system, user, model, mode, schema, options...)
}

func TestBuildStepsReadsArgumentsFromItsSelectedArchitecture(t *testing.T) {
	reply := &argumentGroupingReplies{}
	deps := groupingHost(t, reply)
	deps.TmpDir = t.TempDir()

	const section = "step_arguments:\n- {name: regex, read: Whole tail., applies: stdout, escaping: none}\n"

	body, err := disk.Read("docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]any

	err = yaml.Unmarshal(body, &document)
	if err != nil {
		t.Fatal(err)
	}

	body, err = yaml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("selected-architecture.yaml", append(body, []byte(section)...), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "selected-architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	if len(reply.arguments) == 0 {
		t.Fatal("no concept turns")
	}

	for _, arguments := range reply.arguments {
		if arguments != section {
			t.Fatalf("selected architecture evidence lost: %q", arguments)
		}
	}
}
