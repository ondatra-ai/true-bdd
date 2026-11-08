package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestUnusedPatternFixtureIsRefusedThenAcceptsTheForwardedRepair(t *testing.T) {
	fixture, err := filepath.Abs("../../../../../tests/true-bdd/fixtures/concept-steps-unused/cli/claude")
	if err != nil {
		t.Fatal(err)
	}

	root, programs := t.TempDir(), t.TempDir()
	t.Setenv("PATH", programs+":/usr/bin:/bin")

	first := unusedFixturePatterns(t, fixture, root)
	steps := []catalogue.SuiteStep{
		{Block: catalogue.BlockWhen, Text: "the Owner halves\u00a06", Item: repairConcept},
		{Block: catalogue.BlockWhen, Text: "the Owner halves\u00a0-7", Item: repairConcept},
		{Block: catalogue.BlockThen, Text: "the result is 3", Bound: "result-definition"},
		{Block: catalogue.BlockThen, Text: "the result is -3", Bound: "result-definition"},
	}

	findings := catalogue.CheckPatterns(repairConcept, first, steps)
	if len(findings) != 1 || !strings.Contains(findings[0], "binds no step of the suite") {
		t.Fatalf("the first proposal must fail only the unused-pattern check: %v", findings)
	}

	response := map[string]any{"structured_output": map[string]any{"patterns": first[:1]}}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	program := filepath.Join(programs, "claude")

	err = disk.Write(program, []byte("#!/bin/sh\ncat <<'JSON'\n"+string(data)+"\nJSON\n"), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(program, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	second := unusedFixturePatterns(t, fixture, root)
	if findings = catalogue.CheckPatterns(repairConcept, second, steps); len(findings) != 0 {
		t.Fatalf("the forwarded repair must pass the same check: %v", findings)
	}
}

func unusedFixturePatterns(t *testing.T, fixture, root string) []catalogue.ProposedPattern {
	t.Helper()

	command := exec.CommandContext(t.Context(), fixture, "--system-prompt",
		"Propose the step definition patterns of one concept for one suite,")
	command.Dir = root

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture CLI: %s, %v", output, err)
	}

	var response struct {
		StructuredOutput struct {
			Patterns []catalogue.ProposedPattern `json:"patterns"`
		} `json:"structured_output"`
	}

	err = json.Unmarshal(output, &response)
	if err != nil {
		t.Fatal(err)
	}

	return response.StructuredOutput.Patterns
}
