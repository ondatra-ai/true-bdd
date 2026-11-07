package commands_test

import (
	"context"
	"encoding/json"
	"maps"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const otherSuiteService = "other"

type suiteReplies struct{ verificationReplies }

func (reply *suiteReplies) ExecutePromptWithSystem(ctx context.Context, role provider.Role, system, user string,
	model provider.ModelRef, mode provider.ExecutionMode, schema string, options ...provider.TurnOptions,
) (string, error) {
	if role == provider.RoleApply && strings.Contains(user, "Suite: tests/other/steps") {
		reply.applies++

		return writeVerificationSuiteBodies(user, "tests/other/steps")
	}

	return reply.verificationReplies.ExecutePromptWithSystem(ctx, role, system, user, model, mode, schema, options...)
}

func TestBuildStepsWritesTwoFreshSuitesInChecklistOrder(t *testing.T) {
	reply := &suiteReplies{verificationReplies: verificationReplies{conceptReplies: conceptReplies{
		proposals: []string{patternAnswer(`^Press Halve once for (\w+)\.$`, true)},
	}}}
	deps := conceptHost(t, reply)
	prepareVerificationWalk(t, &deps, reply)

	source, err := disk.Read("docs/scenarios.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]map[string]map[string]any

	err = json.Unmarshal(source, &document)
	if err != nil {
		t.Fatal(err)
	}

	second := make(map[string]any)
	for key, value := range document["scenarios"]["host-001"] {
		second[key] = value
	}

	second["path"] = "tests/other/other_test.go"
	second["service"] = otherSuiteService

	duplicateConceptService(t)

	document["scenarios"]["host-002"] = second

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("docs/scenarios.yaml", encoded, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil || reply.applies != 2 {
		t.Fatalf("two suites: %v; body applies=%d", err, reply.applies)
	}
}

func duplicateConceptService(t *testing.T) {
	t.Helper()

	const architecturePath = "docs/architecture/architecture.yaml"

	source, err := disk.Read(architecturePath)
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]map[string]json.RawMessage

	err = json.Unmarshal(source, &document)
	if err != nil {
		t.Fatal(err)
	}

	arch := document["architecture"]
	arch["services"] = duplicateArchitectureEntries(t, arch["services"],
		map[string]string{"name": otherSuiteService, "path": "services/other"})
	arch["concepts"] = duplicateArchitectureEntries(t, arch["concepts"],
		map[string]string{"service": otherSuiteService})

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write(architecturePath, encoded, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func duplicateArchitectureEntries(t *testing.T, raw json.RawMessage, changes map[string]string) json.RawMessage {
	t.Helper()

	var entries []map[string]json.RawMessage

	err := json.Unmarshal(raw, &entries)
	if err != nil {
		t.Fatal(err)
	}

	duplicated := append([]map[string]json.RawMessage{}, entries...)
	for _, entry := range entries {
		duplicate := maps.Clone(entry)
		for key, value := range changes {
			duplicate[key] = json.RawMessage(strconv.Quote(value))
		}

		duplicated = append(duplicated, duplicate)
	}

	encoded, err := json.Marshal(duplicated)
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}
