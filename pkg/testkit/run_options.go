package testkit

import (
	"context"
	"errors"
	"path/filepath"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// RunOptions selects a wrapper executable and environment overrides for a run.
// Empty Binary uses the suite's lazily built service.
type RunOptions struct {
	Binary string
	Env    []string
}

var (
	ErrNoTreePrepared = errors.New("no Given step prepared a project tree")
	ErrNoRun          = errors.New("no When step ran the CLI")
)

// PrepareTree loads fixture data and resolves both callers' recording shelves.
func (s *ScenarioRun) PrepareTree(name string) error {
	fixture, err := runner.LoadFixture(s.Harness.FixtureDir(name))
	if err != nil {
		return s.fail("loading the %q tree: %w", name, err)
	}

	s.Fixture = fixture

	proxy, err := s.resolveProxy(name)
	if err != nil {
		return err
	}

	s.Proxy = proxy

	return nil
}

// Run executes under the proxy, preserving results even on infrastructure failure.
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
		err = s.Harness.RequireEngine()
		if err != nil {
			return s.fail("%w", err)
		}

		binary = s.Harness.BinPath
	}

	timeout := s.runTimeout()
	s.T.Logf("running %q (%s, timeout %s) — this can take several minutes", command, s.Fixture.Name, timeout)
	env := append(slices.Clone(s.Proxy.Env), options.Env...)
	result, err := runner.Execute(context.Background(), s.Fixture, binary, s.Harness.SessionRoot, timeout, env...)
	s.Recorder.ObserveRun(result, err)

	s.Result = result
	if err != nil {
		return s.fail("executing %q: %w", command, err)
	}

	return nil
}

// ObserveServices arms a live caller for fixture-controlled CLI observation.
func (s *ScenarioRun) ObserveServices(fixture string) {
	if s.Harness.Modes.Services != runner.ProxyModeLive {
		return
	}

	s.Proxy.StateDir = filepath.Join(runner.RunDir(s.Harness.SessionRoot, s.Fixture.Name), "tmp", "aiproxy-state")
	s.Proxy.Env = runner.AIProxyEnv("observe", s.Harness.Shims.Services, fixture, s.Proxy.StateDir, s.Harness.Shims.All())
}
