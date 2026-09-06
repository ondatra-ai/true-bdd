package stepcoverage_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/stepcoverage"
)

// plant writes a steps package under <root>/steps/ inside a fresh repo
// root, and returns that root.
func plant(t *testing.T, trees map[string]string) string {
	t.Helper()

	repoRoot := t.TempDir()

	for root, body := range trees {
		dir := filepath.Join(repoRoot, filepath.FromSlash(root), "steps")

		err := os.MkdirAll(dir, 0o750)
		if err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		writeErr := os.WriteFile(filepath.Join(dir, "steps.go"), []byte(body), 0o600)
		if writeErr != nil {
			t.Fatalf("write: %v", writeErr)
		}
	}

	return repoRoot
}

// scenario builds one registry scenario from its statements.
func scenario(id, service, path string, statements ...registry.Statement) *registry.RegistryScenario {
	return &registry.RegistryScenario{
		ID: id, Service: service, Path: path, Statements: statements,
	}
}

// stepText is the one step text these scenarios declare, and mcpRoot the
// suite root most of them own.
const (
	stepText = "it works"
	mcpRoot  = "tests/mcp"
)

// step is the deterministic Given every scenario here carries.
func step() registry.Statement {
	return registry.Statement{
		Keyword: registry.KeywordGiven, Mode: registry.ModeDeterministic, Text: stepText,
	}
}

// A scenario whose every step binds is examined and carries no gap.
func TestScanBindsEveryStep(t *testing.T) {
	t.Parallel()

	repoRoot := plant(t, map[string]string{
		mcpRoot: "package steps\n\nfunc Register(suite Suite) {\nsuite.Step(`^it works$`, nil)\n}\n",
	})

	answer, err := stepcoverage.Scan(
		[]*registry.RegistryScenario{
			scenario("INT-900", "mcp-service", mcpRoot+"/mcp_test.go", step()),
		}, repoRoot)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if !answer.Examined["INT-900"] {
		t.Error("want the scenario examined")
	}

	if len(answer.Gaps) != 0 {
		t.Errorf("want no gaps, got %v", answer.Gaps)
	}
}

// An absent steps/ tree is zero definitions: every step is a gap, and
// the scenario is STILL examined — the shape that keeps a host with no
// definitions yet walking every scenario it declares.
func TestScanTreatsAbsentTreeAsAllGaps(t *testing.T) {
	t.Parallel()

	answer, err := stepcoverage.Scan(
		[]*registry.RegistryScenario{
			scenario("INT-900", "mcp-service", mcpRoot+"/mcp_test.go", step()),
		}, t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if !answer.Examined["INT-900"] {
		t.Error("want the scenario examined even with no definitions")
	}

	if len(answer.Gaps["INT-900"]) != 1 {
		t.Errorf("want 1 gap, got %v", answer.Gaps["INT-900"])
	}
}

// A model-run step binds by construction and must never be a gap.
func TestScanNeverGapsModelRunSteps(t *testing.T) {
	t.Parallel()

	answer, err := stepcoverage.Scan(
		[]*registry.RegistryScenario{
			scenario("INT-900", "mcp-service", mcpRoot+"/mcp_test.go",
				registry.Statement{Keyword: registry.KeywordWhen, Mode: registry.ModeAct, Text: "llm: do it"},
				registry.Statement{Keyword: registry.KeywordThen, Mode: registry.ModeRule, Text: "judge: it reads right"},
			),
		}, t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(answer.Gaps) != 0 {
		t.Errorf("want no gaps, got %v", answer.Gaps)
	}
}

// Two definitions matching one step refuse the whole answer rather than
// reporting a gap a fix turn would close with a third.
func TestScanRefusesAmbiguousStep(t *testing.T) {
	t.Parallel()

	repoRoot := plant(t, map[string]string{
		mcpRoot: "package steps\n\nfunc Register(suite Suite) {\n" +
			"suite.Step(`^it works$`, nil)\nsuite.Step(`^it (.+)$`, nil)\n}\n",
	})

	answer, err := stepcoverage.Scan(
		[]*registry.RegistryScenario{
			scenario("INT-900", "mcp-service", mcpRoot+"/mcp_test.go", step()),
		}, repoRoot)
	if !errors.Is(err, stepcoverage.ErrAmbiguousStep) {
		t.Fatalf("want ErrAmbiguousStep, got %v", err)
	}

	if answer != nil {
		t.Error("want no partial answer alongside the refusal")
	}
}

// One service's definitions must not bind another's steps.
func TestScanKeepsServicesApart(t *testing.T) {
	t.Parallel()

	repoRoot := plant(t, map[string]string{
		mcpRoot:     "package steps\n\nfunc Register(suite Suite) {\nsuite.Step(`^it works$`, nil)\n}\n",
		"tests/web": "package steps\n\nfunc Register(suite Suite) {\nsuite.Step(`^unrelated$`, nil)\n}\n",
	})

	answer, err := stepcoverage.Scan(
		[]*registry.RegistryScenario{
			scenario("INT-900", "mcp-service", mcpRoot+"/mcp_test.go", step()),
			scenario("INT-901", "web-service", "tests/web/web_test.go", step()),
		}, repoRoot)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(answer.Gaps["INT-900"]) != 0 {
		t.Errorf("mcp binds its own step, got %v", answer.Gaps["INT-900"])
	}

	if len(answer.Gaps["INT-901"]) != 1 {
		t.Errorf("web must not borrow mcp's definition, got %v", answer.Gaps["INT-901"])
	}
}

// A service whose scenarios name two roots has no single steps/ tree.
func TestScanRefusesTwoRootsForOneService(t *testing.T) {
	t.Parallel()

	_, err := stepcoverage.Scan(
		[]*registry.RegistryScenario{
			scenario("INT-900", "mcp-service", mcpRoot+"/mcp_test.go", step()),
			scenario("INT-901", "mcp-service", "tests/other/other_test.go", step()),
		}, t.TempDir())
	if !errors.Is(err, stepcoverage.ErrTwoRootsForOneService) {
		t.Fatalf("want ErrTwoRootsForOneService, got %v", err)
	}
}

// Every suite's table is read before any step resolves, so a pattern
// that will not fold beats an ambiguity to the refusal on every run —
// whichever service happens to hold it.
func TestScanReportsTheUnreadablePatternFirst(t *testing.T) {
	t.Parallel()

	repoRoot := plant(t, map[string]string{
		"tests/aaa": "package steps\n\nfunc Register(suite Suite) {\n" +
			"suite.Step(`^it works$`, nil)\nsuite.Step(`^it (.+)$`, nil)\n}\n",
		"tests/zzz": "package steps\n\nfunc Register(suite Suite) {\nsuite.Step(computed(), nil)\n}\n",
	})

	scenarios := []*registry.RegistryScenario{
		scenario("INT-900", "aaa-service", "tests/aaa/aaa_test.go", step()),
		scenario("INT-901", "zzz-service", "tests/zzz/zzz_test.go", step()),
	}

	for range 20 {
		_, err := stepcoverage.Scan(scenarios, repoRoot)
		if !errors.Is(err, stepcoverage.ErrPatternNotConstant) {
			t.Fatalf("want ErrPatternNotConstant to win every run, got %v", err)
		}
	}
}
