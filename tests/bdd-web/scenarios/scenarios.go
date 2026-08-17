//go:build bdd

// Package scenarios binds bddgo's generics to the bdd-web suite.
//
// The same shape as the CLI suite's shim, and deliberately so: the two
// suites differ in what they drive, not in what they are. What differs
// here is the cost of coming up — a Next.js build, a server, a browser —
// which is exactly why the harness is lazy. Asking whether the registry
// and the generated files agree must not require chromium.
package scenarios

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/bdd-web/steps"
	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// -mode mirrors the CLI suite's flag so one architectural spec can
// declare all three commands for both suites in the same vocabulary.
// The web scenarios reach no model yet, so today the value only travels
// with the run for the report to state.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var proxyMode = flag.String("mode", runner.ProxyModeLive,
	"AI CLI mode: live, record, or replay")

// -allow-missing-toolchain downgrades a missing node/npm from a failure
// to a skip.
//
// Off by default, and that is the whole point of the flag existing. A
// suite that silently skips when a tool is absent reports the same green
// as one that ran, and this suite is in no gate to catch the difference.
// Someone who genuinely wants the skip — a laptop without node, a
// half-provisioned container — says so on the command line, where it is
// visible in the invocation rather than invisible in the output.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var allowMissingToolchain = flag.Bool("allow-missing-toolchain", false,
	"skip instead of failing when node/npm are not installed")

// toolchainMissingMarker heads the failure a missing toolchain produces,
// so a CI log search finds it without parsing Go test output.
const toolchainMissingMarker = "BDD-WEB: TOOLCHAIN MISSING"

//nolint:gochecknoglobals // process-wide suite state, built once in Main
var (
	suite    *bddgo.Suite[steps.State]
	repoRoot string

	bootOnce  sync.Once
	bootErr   error
	skipWhy   string
	teardowns []func()
)

// Main is the suite's TestMain body. It returns rather than exits so the
// caller can os.Exit after the teardowns have run.
func Main(m *testing.M) int {
	flag.Parse()

	mode := *proxyMode
	if mode != runner.ProxyModeLive && mode != runner.ProxyModeRecord && mode != runner.ProxyModeReplay {
		fmt.Fprintf(os.Stderr, "invalid -mode %q: want live, record, or replay\n", mode)

		return 1
	}

	var err error

	repoRoot, err = runner.FindRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)

		return 1
	}

	suite, err = bddgo.New[steps.State](steps.Options(repoRoot))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)

		return 1
	}

	steps.Register(suite)

	code := m.Run()

	for index := len(teardowns) - 1; index >= 0; index-- {
		teardowns[index]()
	}

	return code
}

// New returns the Run a generated test drives, bringing the application
// under test up on the first call.
func New(t *testing.T, scenarioID string) *bddgo.Run[steps.State] {
	t.Helper()

	ensureHarness(t)

	return suite.Scenario(t, scenarioID)
}

// CheckCoverage refuses a repository whose registry and generated tests
// disagree.
func CheckCoverage(t *testing.T) {
	t.Helper()

	suite.CheckCoverage(t, ".", repoRoot)
}

// CheckStepCoverage reports every registry step that binds to no step
// definition, and writes the report `build tests` reads.
func CheckStepCoverage(t *testing.T) {
	t.Helper()

	suite.ReportStepCoverage(t)
}

func ensureHarness(t *testing.T) {
	t.Helper()

	bootOnce.Do(func() { skipWhy, bootErr = bootHarness(t) })

	if bootErr != nil {
		t.Fatalf("bring the suite up: %v", bootErr)
	}

	if skipWhy != "" {
		t.Skip(skipWhy)
	}
}

// bootHarness builds the application, boots it, and opens a browser. A
// non-empty first result is a reason to skip; a non-nil second is a
// failure.
func bootHarness(t *testing.T) (string, error) {
	missing := missingTools()
	if len(missing) > 0 {
		reason := fmt.Sprintf("%s: %s — the bdd-web suite cannot build its subject",
			toolchainMissingMarker, strings.Join(missing, ", "))

		if *allowMissingToolchain {
			return reason, nil
		}

		return "", fmt.Errorf("%s\n  pass -allow-missing-toolchain to skip instead", reason)
	}

	harness, stop, err := steps.NewHarness(context.Background(), *proxyMode, repoRoot)
	if err != nil {
		return "", fmt.Errorf("bring up the application under test: %w", err)
	}

	teardowns = append(teardowns, stop)
	t.Logf("application under test: %s", harness.BaseURL)

	suite.Init(steps.NewState(harness))

	return "", nil
}

// missingTools names every tool the suite needs and cannot find.
//
// All of them, not the first: a machine missing node is missing npm too,
// and reporting one at a time turns provisioning into a guessing game.
func missingTools() []string {
	var missing []string

	for _, tool := range []string{"node", "npm"} {
		_, err := exec.LookPath(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}

	return missing
}
