package steps_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/steps"
)

// The record hint must name the test `build tests` generated, so the mirror
// CamelCases a key exactly as the engine's renderer does (ADR 0014): this is
// the table the engine's TestFuncNameCamelCasesTheKey pins.
func TestTestNameCamelCasesTheKey(t *testing.T) {
	t.Parallel()

	for key, want := range map[string]string{
		"E2E-021":                           "TestE2E021",
		"INT-900":                           "TestINT900",
		"int-900":                           "TestInt900",
		"96.2-001-foo-bar":                  "Test962001FooBar",
		"mcp-endpoint-001-initialize-ok":    "TestMcpEndpoint001InitializeOk",
		"scen-check-001-unknown-id-refused": "TestScenCheck001UnknownIdRefused",
	} {
		got := steps.TestName(bddgo.Scenario{ID: key})
		if got != want {
			t.Errorf("TestName(%q) = %q, want %q", key, got, want)
		}
	}
}

// treeScenario names its project tree the way every bdd-cli Given step does.
func treeScenario(tree string) bddgo.Scenario {
	return bddgo.Scenario{ID: "E2E-900", Steps: []bddgo.Step{
		{Keyword: "Given", Mode: bddgo.ModeAct, Text: `the "` + tree + `" project tree`},
	}}
}

// A step definition may resolve its recordings before it keeps the tree it
// loaded (#140): a record run whose State holds no Fixture still publishes,
// under the name its Given step gave the tree.
func TestARecordRunWithoutAFixturePublishesUnderTheTreesName(t *testing.T) {
	t.Parallel()

	harness := &steps.Harness{
		SessionRoot: t.TempDir(),
		FixturesDir: t.TempDir(),
		Modes:       runner.Modes{Services: runner.ProxyModeRecord, Tests: runner.ProxyModeReplay},
	}

	err := os.Mkdir(harness.FixtureDir("kept-nowhere"), 0o755)
	if err != nil {
		t.Fatalf("create the fixture tree: %v", err)
	}

	staging := filepath.Join(t.TempDir(), "staging")

	err = os.Mkdir(staging, 0o755)
	if err != nil {
		t.Fatalf("create the staging dir: %v", err)
	}

	// Registered before NewState registers the State's own cleanup, so it
	// runs after the State has published (cleanups run last-in, first-out).
	t.Cleanup(func() { assertPublishedAs(t, harness, "kept-nowhere") })

	state, err := steps.NewState(harness)(&bddgo.World{T: t, Scenario: treeScenario("kept-nowhere")})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}

	state.Proxy = steps.ProxySetup{Staging: staging, Cassettes: staging}
	state.Result = &runner.RunResult{}
}

// assertPublishedAs checks a record run's cassettes reached the named tree.
func assertPublishedAs(t *testing.T, harness *steps.Harness, tree string) {
	t.Helper()

	cassettes, err := harness.CassetteDir(tree)
	if err != nil {
		t.Errorf("CassetteDir: %v", err)

		return
	}

	golden, err := runner.ReadGolden(cassettes)
	if err != nil {
		t.Errorf("the recording was not published under the tree's name: %v", err)

		return
	}

	if golden.Fixture != tree {
		t.Errorf("golden names fixture %q, want %q", golden.Fixture, tree)
	}
}

// The judge rules on the command the When step ran, which only the tree
// carries: a State that ran without one is refused, never dereferenced.
func TestTheJudgeRefusesAStateThatRanWithoutAFixture(t *testing.T) {
	t.Parallel()

	state := &steps.State{
		T:        t,
		Scenario: treeScenario("kept-nowhere"),
		Harness:  &steps.Harness{JudgeTimeout: time.Second},
		Result:   &runner.RunResult{},
	}

	err := state.Judge([]bddgo.Step{{Keyword: "Then", Mode: bddgo.ModeRule, Text: "the story reads well"}})
	if !errors.Is(err, steps.ErrNoTreePrepared) {
		t.Fatalf("Judge without a fixture = %v, want %v", err, steps.ErrNoTreePrepared)
	}
}
