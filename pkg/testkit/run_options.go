package testkit

import (
	"context"
	"errors"
	"path/filepath"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

// RunOptions selects a wrapper executable and environment overrides for a run.
type RunOptions struct {
	// Binary selects a wrapper executable, defaulting to the suite's lazily built service.
	Binary string
	// Env overrides environment entries for this invocation.
	Env []string
}

var (
	// ErrNoTreePrepared marks a run requested before a Given prepared its tree.
	ErrNoTreePrepared = errors.New("no Given step prepared a project tree")
	// ErrNoRun marks an observation requested before a When ran the service.
	ErrNoRun = errors.New("no When step ran the CLI")
)

// PrepareTree selects the fixture whose input and scripts describe this Scenario's starting state.
func (s *ScenarioRun) PrepareTree(name string) error {
	fixture, err := runner.LoadFixture(s.harness.FixtureDir(name))
	if err != nil {
		return s.fail("loading the %q tree: %w", name, err)
	}

	s.Fixture = fixtureView(fixture)

	proxy, err := s.resolveProxy(name)
	if err != nil {
		return err
	}

	s.proxy = proxy

	return nil
}

// Run invokes the service in the assembled fixture tree, retaining outputs and file changes even on failure.
func (s *ScenarioRun) Run(command string, options RunOptions) error {
	if s.Fixture == nil {
		return s.fail("%w", ErrNoTreePrepared)
	}

	err := s.Fixture.UseCommand(command)
	if err != nil {
		return s.fail("%w", err)
	}

	binary := options.Binary
	if binary == "" {
		err = s.harness.RequireEngine()
		if err != nil {
			return s.fail("%w", err)
		}

		binary = s.harness.BinPath
	}

	timeout := s.runTimeout()
	s.T.Logf("running %q (%s, timeout %s) — this can take several minutes", command, s.Fixture.Name, timeout)
	env := append(slices.Clone(s.proxy.Env), options.Env...)
	fixture := fixtureToRunner(s.Fixture)
	fixture.RepoLayer = s.harness.RepoLayer
	fixture.BeforeSnapshot = s.beforeSnapshot
	fixture.OmitPath = s.omittedPath
	result, err := runner.Execute(context.Background(), fixture, binary, s.harness.SessionRoot, timeout, env...)
	s.recorder.ObserveRun(result, err)

	s.Result = resultView(result, s.harness.BinPath)
	if err != nil {
		return s.fail("executing %q: %w", command, err)
	}

	return nil
}

// ObserveServices ensures that this Scenario's service CLI spawns are observed even in live mode.
func (s *ScenarioRun) ObserveServices(fixture string) {
	if s.harness.Modes.Services != runner.ProxyModeLive {
		return
	}

	s.proxy.StateDir = filepath.Join(runner.RunDir(s.harness.SessionRoot, s.Fixture.Name), "tmp", "aiproxy-state")
	s.proxy.Env = runner.AIProxyEnv("observe", s.harness.Shims.Services, fixture, s.proxy.StateDir, s.harness.Shims.All())
}
