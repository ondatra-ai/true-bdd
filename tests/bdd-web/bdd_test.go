//go:build bdd

package bddweb_test

import (
	"flag"
	"os/exec"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-web/steps"
	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// -mode mirrors the CLI suite's flag so one architectural spec can
// declare all three commands for both suites in the same vocabulary.
// The web scenarios reach no model yet, so today the value only travels
// with the run for the report to state; it is here rather than added
// later because a suite whose modes appear halfway through its life
// leaves a spec that lied about it until then.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var proxyMode = flag.String("mode", runner.ProxyModeLive,
	"AI CLI mode: live, record, or replay")

// TestBDDFixtures runs every scenario in docs/scenarios.yaml that the
// architectural spec assigns to the bdd-web suite.
//
// Named identically to the CLI suite's entry point on purpose: the two
// suites differ in what they drive, not in what they are, and one
// `-run 'TestBDDFixtures/<name>'` filter should mean the same thing in
// either.
func TestBDDFixtures(t *testing.T) {
	mode := *proxyMode
	if mode != runner.ProxyModeLive && mode != runner.ProxyModeRecord && mode != runner.ProxyModeReplay {
		t.Fatalf("invalid -mode %q: want live, record, or replay", mode)
	}

	requireToolchain(t)

	repoRoot, err := runner.FindRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	harness, stop, err := steps.NewHarness(t.Context(), mode, repoRoot)
	if err != nil {
		t.Fatalf("bring up the application under test: %v", err)
	}

	t.Cleanup(stop)
	t.Logf("application under test: %s", harness.BaseURL)

	suite := bddgo.New[steps.State](t, steps.Options(repoRoot))
	suite.Init(steps.NewState(harness))
	steps.Register(suite)

	suite.Run()
}

// requireToolchain skips when the machine cannot build or serve the
// application at all.
//
// A skip here and not a failure, unlike everywhere else in this
// repository: node's absence says nothing about the code under test,
// and this suite is not in any gate — the run that matters is a
// deliberate one on a machine that has the toolchain. What must never
// be skipped is a scenario, and none is.
func requireToolchain(t *testing.T) {
	t.Helper()

	for _, tool := range []string{"node", "npm"} {
		_, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("`%s` is not on $PATH; the bdd-web suite cannot build its subject: %v", tool, err)
		}
	}
}
