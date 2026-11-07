package architecture_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestLoadStepArgumentsPreservesCompleteSection(t *testing.T) {
	const section = `step_arguments:
  - name: regex
    read: |
      Compile the whole tail, including quotes.
      Keep {{CWD}} literal.
    applies: stdout and stderr
    escaping: none
`

	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := disk.Write(path, []byte("architecture: {}\n"+section+"other: value\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	got, err := architecture.LoadStepArguments(path)
	if err != nil {
		t.Fatal(err)
	}

	if got != section {
		t.Fatalf("section lost evidence: %q", got)
	}
}

func TestLoadStepArgumentsAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := disk.Write(path, []byte("architecture: {}\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	got, err := architecture.LoadStepArguments(path)
	if err != nil || got != "" {
		t.Fatalf("absent section: %q, %v", got, err)
	}
}

func TestLoadStepArgumentsFromFlowDocuments(t *testing.T) {
	const source = `{"architecture":{"private":"unrelated-binding"},"step_arguments":[` +
		`{"name":"regex","read":"Whole tail, including quotes.","applies":"stdout","escaping":"none"}]}`

	path := filepath.Join(t.TempDir(), "architecture.json")

	err := disk.Write(path, []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	got, err := architecture.LoadStepArguments(path)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Whole tail, including quotes.") || strings.Contains(got, "unrelated-binding") {
		t.Fatalf("flow document must supply only complete argument evidence: %s", got)
	}
}

func TestLoadStepArgumentsResolvesAliasedEvidence(t *testing.T) {
	const source = "rule: &rule Whole tail, including quotes.\narchitecture: {}\n" +
		"step_arguments:\n- {name: regex, read: *rule, applies: stdout, escaping: none}\n"

	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := disk.Write(path, []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	got, err := architecture.LoadStepArguments(path)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Whole tail, including quotes.") || strings.Contains(got, "*rule") {
		t.Fatalf("aliased evidence must carry its complete value: %s", got)
	}
}
