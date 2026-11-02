package testkit

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
	"log/slog"
)

const (
	scenarioTimeout = 5 * time.Minute
	judgeTimeout    = 5 * time.Minute
	buildTimeout    = 2 * time.Minute
)

// Suite owns TestMain lifecycle and lazily brings up the harness.
// NewSuite registers the harness flags; construct adds definition-owned State fields.
type Suite[S any] struct {
	name      string
	aiCLIs    []string
	flags     *Flags
	suite     *bddgo.Suite[S]
	register  func(*bddgo.Suite[S])
	construct func(*ScenarioRun) *S
	repoRoot  string
	config    hostConfig
	runModes  runner.Modes
	bootOnce  sync.Once
	errBoot   error
	skipWhy   string
	harness   *Harness
	teardowns []func()
}

// NewSuite creates Suite wiring without reading files or building binaries.
func NewSuite[S any](
	name string, aiCLIs []string, register func(*bddgo.Suite[S]), construct func(*ScenarioRun) *S,
) *Suite[S] {
	return &Suite[S]{
		name: name, aiCLIs: aiCLIs, flags: RegisterFlags(flag.CommandLine), register: register, construct: construct,
	}
}

// Main is the suite's TestMain body. It returns instead of calling os.Exit
// directly, so the caller can run deferred teardowns first — TestMain has
// no *testing.T, so t.Cleanup is unavailable.
func (h *Suite[S]) Main(m *testing.M) int {
	logging.Install(logging.Stderr, "", "true-bdd")

	// Parsed explicitly: m.Run parses too late for a flag Main reads.
	flag.Parse()

	// A supplied value must be readable; an absent one is legal here and
	// refused at boot, since the coverage guards run with no mode at all.
	modes, err := h.flags.Modes()
	if err != nil {
		slog.Error("invalid -mode", "error", err)

		return 1
	}

	h.runModes = modes

	h.repoRoot, err = runner.FindRepoRoot()
	if err != nil {
		slog.Error("find repo root", "error", err)

		return 1
	}

	h.config, err = loadHostConfig(h.repoRoot)
	if err != nil {
		slog.Error("read host configuration", "error", err)

		return 1
	}

	h.suite, err = bddgo.New[S](bddgo.Options{
		Registry: h.config.registry, Architecture: h.config.architecture, Suite: h.name,
	})
	if err != nil {
		slog.Error("build suite", "error", err)

		return 1
	}

	h.register(h.suite)

	code := m.Run()

	for index := len(h.teardowns) - 1; index >= 0; index-- {
		h.teardowns[index]()
	}

	return code
}

// New returns the Run a generated test drives, bringing the harness up
// lazily on the first call — eager init here would cost every coverage
// guard a binary build and an aiproxy shim just to compare two files.
func (h *Suite[S]) New(t *testing.T, scenarioID string) *bddgo.Run[S] {
	t.Helper()

	h.ensureHarness(t)

	return h.suite.Scenario(t, scenarioID)
}

// CheckCoverage refuses a repository whose registry and generated tests
// disagree. Hand-written on purpose: a generated guard is one the
// generator could silence by regenerating it.
func (h *Suite[S]) CheckCoverage(t *testing.T) {
	t.Helper()

	h.suite.CheckCoverage(t, ".", h.repoRoot)
}

// CheckFixtureTrees refuses a fixture tree no scenario names.
func (h *Suite[S]) CheckFixtureTrees(t *testing.T) {
	t.Helper()

	h.suite.CheckFixtureTrees(t, filepath.Join(h.repoRoot, "tests", h.name, "fixtures"), FixtureName)
}

// ensureHarness builds everything a scenario shares, once.
func (h *Suite[S]) ensureHarness(t *testing.T) {
	t.Helper()

	h.bootOnce.Do(func() { h.skipWhy, h.errBoot = h.bootHarness(t) })

	if h.errBoot != nil {
		t.Fatalf("bring the suite up: %v", h.errBoot)
	}

	// A skip per scenario rather than one for the whole suite: `go test`
	// then reports how many scenarios did not run and why, instead of a
	// single skipped parent that hides the count.
	if h.skipWhy != "" {
		t.Skip(h.skipWhy)
	}
}

// bootHarness assembles the lazy service build, session root, shim, the
// judge and this process's log sink. A non-empty first result is a
// reason to skip; a non-nil second is a failure.
func (h *Suite[S]) bootHarness(t *testing.T) (string, error) {
	t.Helper()

	err := h.flags.RequireMode()
	if err != nil {
		return "", err
	}

	why, err := h.requiredCLIs(h.runModes)
	if err != nil {
		return "", err
	}

	if why != "" {
		return why, nil
	}

	shims, err := h.installShims(h.runModes)
	if err != nil {
		return "", err
	}

	judge, err := runner.NewClaudeJudge(
		h.config.path, h.runModes.Tests)
	if err != nil {
		return "", fmt.Errorf("init judge: %w", err)
	}

	sessionRoot, err := runner.NewSessionRoot()
	if err != nil {
		return "", fmt.Errorf("create session root: %w", err)
	}

	t.Logf("BDD session root: %s", sessionRoot)

	// Point this process's slog at the session before the first judge
	// call. Left unconfigured, slog falls back to the log package's text
	// format on stderr and the judge's cost exists structurally nowhere.
	usage, closeLog, err := runner.InstallHarnessLogging(sessionRoot)
	if err != nil {
		return "", fmt.Errorf("install harness logging: %w", err)
	}

	h.teardowns = append(h.teardowns, closeLog)

	h.harness = &Harness{
		Binary:       runner.NewLazyBinary(h.buildService),
		SessionRoot:  sessionRoot,
		Shims:        shims,
		Modes:        h.runModes,
		Judge:        judge,
		FixturesDir:  filepath.Join(h.repoRoot, "tests", h.name, "fixtures"),
		Timeout:      scenarioTimeout,
		JudgeTimeout: judgeTimeout,
		Usage:        usage,
	}

	h.suite.Init(func(world *bddgo.World) (*S, error) {
		run, stateErr := NewScenarioRun(h.harness)(world)
		if stateErr != nil {
			return nil, stateErr
		}

		return h.construct(run), nil
	})

	return "", h.writeSessionMeta(sessionRoot, h.runModes.String())
}

// writeSessionMeta records what this invocation set out to run, before
// the first scenario starts, so a live report's denominator is the run's
// intent rather than its progress.
func (h *Suite[S]) writeSessionMeta(sessionRoot, mode string) error {
	calls, err := bddgo.ScanScenarioCalls(".")
	if err != nil {
		return fmt.Errorf("scan generated tests: %w", err)
	}

	names := make([]string, 0, len(calls))
	byFunc := make(map[string]string, len(calls))

	for _, call := range calls {
		names = append(names, call.Func)
		byFunc[call.Func] = call.ID
	}

	planned, err := runner.PlannedTests(names, flagValue("test.run"), flagValue("test.skip"))
	if err != nil {
		return fmt.Errorf("resolve planned scenarios: %w", err)
	}

	// Recorded as fixture names, not function names: the report server,
	// the coverage tool, and the run directories all key on the fixture
	// name — renaming this key would ripple through all three for no gain.
	fixtures := make([]string, 0, len(planned))

	for _, name := range planned {
		scenario, found, lookupErr := h.suite.Lookup(byFunc[name])
		if lookupErr != nil {
			return fmt.Errorf("look up %s: %w", byFunc[name], lookupErr)
		}

		if !found {
			continue
		}

		fixtures = append(fixtures, FixtureName(scenario))
	}

	err = runner.WriteSessionMeta(sessionRoot, mode, fixtures)
	if err != nil {
		return fmt.Errorf("write session meta: %w", err)
	}

	return nil
}

// missingCLI checks the suite-supplied service dependencies before any step runs.
func (h *Suite[S]) missingCLI() string {
	for _, agent := range h.aiCLIs {
		lookErr := cli.Require(agent)
		if lookErr != nil {
			return fmt.Sprintf(
				"service requires `%s`, which is not on $PATH: %v",
				agent, lookErr)
		}
	}

	return ""
}

// installShims builds the aiproxy shim once and installs it under the AI
// CLIs' names, one directory per caller — the shim reads its mode from flat
// env vars, so separate dirs are what let one caller replay as another records.
func (h *Suite[S]) installShims(modes runner.Modes) (runner.ShimDirs, error) {
	base, err := disk.TempDir("", "true-bdd-shim-")
	if err != nil {
		return runner.ShimDirs{}, fmt.Errorf("create shim dir: %w", err)
	}

	h.teardowns = append(h.teardowns, func() { _ = disk.RemoveTree(base) })

	proxyPath := filepath.Join(base, "aiproxy")

	err = h.goBuild(proxyPath, h.repoRoot, "github.com/ondatra-ai/true-bdd/pkg/testkit/aiproxy")
	if err != nil {
		return runner.ShimDirs{}, err
	}

	var dirs runner.ShimDirs

	for caller, mode := range map[string]string{
		runner.CallerServices: modes.Services,
		runner.CallerTests:    modes.Tests,
	} {
		if mode == runner.ProxyModeLive && caller != runner.CallerServices {
			continue
		}

		dir, dirErr := h.installShimNames(base, caller, proxyPath)
		if dirErr != nil {
			return runner.ShimDirs{}, dirErr
		}

		if caller == runner.CallerServices {
			dirs.Services = dir
		} else {
			dirs.Tests = dir
		}
	}

	return dirs, nil
}

// installShimNames symlinks one caller's shim dir.
func (h *Suite[S]) installShimNames(base, caller, proxyPath string) (string, error) {
	dir := filepath.Join(base, caller)

	err := disk.Dir(dir, disk.Shared)
	if err != nil {
		return "", fmt.Errorf("create shim dir for %s: %w", caller, err)
	}

	names := h.aiCLIs
	if caller == runner.CallerTests {
		names = []string{"claude"}
	}

	for _, name := range names {
		err = os.Symlink(proxyPath, filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("install shim %s: %w", name, err)
		}
	}

	return dir, nil
}

func (h *Suite[S]) buildService() (string, error) {
	if h.flags.EngineBin != "" {
		return h.flags.PrebuiltEngine()
	}

	dir, err := disk.TempDir("", "true-bdd-bin-")
	if err != nil {
		return "", fmt.Errorf("create bin dir: %w", err)
	}

	h.teardowns = append(h.teardowns, func() { _ = disk.RemoveTree(dir) })

	binPath := filepath.Join(dir, h.name)

	service, err := loadService(h.config.architecture, h.name)
	if err != nil {
		return "", err
	}

	err = h.goBuild(binPath, filepath.Join(h.repoRoot, service.Path), ".")
	if err != nil {
		return "", err
	}

	return binPath, nil
}

// goBuild resolves library packages through the host's module graph.
func (h *Suite[S]) goBuild(binPath, directory, pkg string) error {
	return gotool.Build(cli.Options{Timeout: buildTimeout}, directory, binPath, pkg)
}

// flagValue reads one of `go test`'s own flags. Absent reads as empty:
// a filter that is not set filters nothing.
func flagValue(name string) string {
	found := flag.Lookup(name)
	if found == nil {
		return ""
	}

	return found.Value.String()
}

// requiredCLIs gates boot on the binaries these modes need, per axis: a
// miss FAILS rather than skips, since a judgeless green hides the gap.
func (h *Suite[S]) requiredCLIs(modes runner.Modes) (string, error) {
	if modes.Tests != runner.ProxyModeReplay {
		err := cli.Require("claude")
		if err != nil {
			return "", fmt.Errorf("tests:%s needs the judge's `claude` CLI: %w", modes.Tests, err)
		}
	}

	// A replaying services caller spawns no real CLI; anything else does.
	if modes.Services == runner.ProxyModeReplay {
		return "", nil
	}

	return h.missingCLI(), nil
}
