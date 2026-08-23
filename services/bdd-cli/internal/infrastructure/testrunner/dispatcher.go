package testrunner

import (
	"context"
	"errors"
	"fmt"
)

// Framework constants used by Dispatcher routing and FailingTest.Framework.
// These mirror the `framework:` values declared in architecture.yaml.
const (
	FrameworkGoTest     = "go-test"
	FrameworkPlaywright = "playwright"
	FrameworkJest       = "jest"
)

// ErrUnknownFramework signals that architecture.yaml declared a test
// framework the Dispatcher cannot route to.
var ErrUnknownFramework = errors.New("unknown test framework")

// Status values shared across runner JSON parsers. Each framework
// happens to use the same lowercase string for a test failure.
const (
	statusFailed   = "failed"
	statusTimedOut = "timedOut"
)

// Config is one test-suite block consumed by a Runner, mirroring
// architecture.TestConfig; conversion happens at the LoadItems boundary
// in commands/build_code.go.
type Config struct {
	Path       string // repo-relative test root (e.g. "tests/integration")
	Framework  string // matches one of the Framework* constants
	ConfigFile string // repo-relative config (e.g. "tests/playwright.config.ts")
	Pattern    string // framework-specific filename glob (informational)

	// Command is the complete command line this suite runs under, verbatim
	// from the spec's `commands:` block — whichever mode the caller
	// selected at the LoadItems boundary.
	Command string
}

// Runner is the framework-agnostic primitive every per-framework runner
// implements: Discover walks every test under Config and returns the
// currently-failing ones; RunOne re-executes one by its framework-native id.
type Runner interface {
	// Discover runs every test under cfg, returning one FailingTest per
	// failure tagged with service/suite/framework. A non-nil error means an
	// infrastructure problem — test failures are values, not errors.
	Discover(ctx context.Context, cfg Config, service, suite string) ([]*FailingTest, error)

	// RunOne re-executes one failing test in isolation by its framework-native
	// TestName, returning pass/fail, raw output (tail-truncated by the
	// caller), and a non-nil error only on infrastructure problems.
	RunOne(ctx context.Context, ft *FailingTest) (passed bool, output string, err error)
}

// Dispatcher routes Discover/RunOne calls to the right framework
// implementation. Built once at bootstrap time; concrete runners are
// registered via NewDispatcher.
type Dispatcher struct {
	byFramework map[string]Runner
}

// NewDispatcher builds a Dispatcher from the supplied runner map, one
// Runner per framework name; a framework absent from the map surfaces
// ErrUnknownFramework at call time.
func NewDispatcher(runners map[string]Runner) *Dispatcher {
	owned := make(map[string]Runner, len(runners))
	for k, v := range runners {
		owned[k] = v
	}

	return &Dispatcher{byFramework: owned}
}

// For looks up the Runner registered for the supplied framework. Wraps
// ErrUnknownFramework with the framework name for caller diagnostics.
func (d *Dispatcher) For(framework string) (Runner, error) {
	r, ok := d.byFramework[framework]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownFramework, framework)
	}

	return r, nil
}
