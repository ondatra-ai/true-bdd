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
	"errors"
	"flag"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/tests/bdd-web/steps"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
	"log/slog"
)

// -mode mirrors the CLI suite's flag; this suite reaches no model yet.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var proxyMode = flag.String("mode", "",
	"per-caller AI CLI mode, e.g. target:replay,tests:replay")

// -allow-missing-toolchain skips instead of failing on missing node/npm; off by default so it can't silently go green.
//
//nolint:gochecknoglobals // test-binary flag; parsed by `go test`
var allowMissingToolchain = flag.Bool("allow-missing-toolchain", false,
	"skip instead of failing when node/npm are not installed")

// toolchainMissingMarker heads the failure a missing toolchain produces,
// so a CI log search finds it without parsing Go test output.
const toolchainMissingMarker = "BDD-WEB: TOOLCHAIN MISSING"

// errToolchainMissing is the refusal a missing toolchain raises when the
// suite was not told it may skip.
var errToolchainMissing = errors.New("toolchain missing")

//nolint:gochecknoglobals // process-wide suite state, built once in Main
var (
	suite    *bddgo.Suite[steps.State]
	repoRoot string
	runModes runner.Modes

	bootOnce  sync.Once
	errBoot   error
	skipWhy   string
	teardowns []func()
)

// Main is the suite's TestMain body. It returns rather than exits so the
// caller can os.Exit after the teardowns have run.
func Main(m *testing.M) int {
	logging.Install(logging.Stderr, "", "bdd-web")

	flag.Parse()

	// Absence is legal, as it is for the CLI suite: the coverage guards
	// pass no mode and bring no harness up.
	modes, err := runner.ParseModes(*proxyMode)
	if err != nil {
		slog.Error("invalid -mode", "error", err)

		return 1
	}

	runModes = modes

	repoRoot, err = runner.FindRepoRoot()
	if err != nil {
		slog.Error("find repo root", "error", err)

		return 1
	}

	suite, err = bddgo.New[steps.State](steps.Options(repoRoot))
	if err != nil {
		slog.Error("build suite", "error", err)

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

	bootOnce.Do(func() { skipWhy, errBoot = bootHarness(t) })

	if errBoot != nil {
		t.Fatalf("bring the suite up: %v", errBoot)
	}

	if skipWhy != "" {
		t.Skip(skipWhy)
	}
}

// bootHarness builds the application, boots it, and opens a browser. A
// non-empty first result is a reason to skip; a non-nil second is a
// failure.
func bootHarness(t *testing.T) (string, error) {
	t.Helper()

	missing := missingTools()
	if len(missing) > 0 {
		reason := fmt.Sprintf("%s: %s — the bdd-web suite cannot build its subject",
			toolchainMissingMarker, strings.Join(missing, ", "))

		if *allowMissingToolchain {
			return reason, nil
		}

		return "", fmt.Errorf("%w: %s\n  pass -allow-missing-toolchain to skip instead",
			errToolchainMissing, reason)
	}

	harness, stop, err := steps.NewHarness(context.Background(), runModes.Target, repoRoot)
	if err != nil {
		return "", fmt.Errorf("bring up the application under test: %w", err)
	}

	teardowns = append(teardowns, stop)

	t.Logf("application under test: %s", harness.BaseURL)

	suite.Init(steps.NewState(harness))

	return "", nil
}

// missingTools names every tool the suite needs and cannot find — all
// of them, not the first, since reporting one at a time turns
// provisioning into a guessing game.
func missingTools() []string {
	var missing []string

	for _, tool := range []string{"node", "npm"} {
		err := cli.Require(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}

	return missing
}
