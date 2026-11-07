package testkit

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
	"os"
	"slices"
	"strings"
)

// SetEnv supplies environment entries for the next service run; later entries win.
func (s *ScenarioRun) SetEnv(entries ...string) { s.proxy.Env = append(s.proxy.Env, entries...) }

// Environment returns the environment entries supplied for the next service run.
func (s *ScenarioRun) Environment() []string { return slices.Clone(s.proxy.Env) }

// ObservationDirectory is where CLI stand-ins retain this Scenario's observations.
func (s *ScenarioRun) ObservationDirectory() string { return s.proxy.StateDir }

// WorkingDirectory is the assembled tree's location, available before the service runs.
func (s *ScenarioRun) WorkingDirectory() string {
	return runner.RunDir(s.harness.SessionRoot, s.Fixture.Name)
}

// Mode is the services caller's current record, replay or live mode.
func (s *ScenarioRun) Mode() string { return s.harness.Modes.Services }

// ServiceBinary resolves and lazily builds this suite's service executable.
func (s *ScenarioRun) ServiceBinary() (string, error) {
	err := s.harness.RequireEngine()
	if err != nil {
		return "", err
	}

	return s.harness.BinPath, nil
}

// BeforeSnapshot registers work after input assembly and before the run's initial snapshot.
func (s *ScenarioRun) BeforeSnapshot(prepare func(string) error) {
	s.beforeSnapshot = append(s.beforeSnapshot, prepare)
}

// PrependPath places directory before every existing entry for the next service run.
func (s *ScenarioRun) PrependPath(directory string) {
	path := os.Getenv("PATH")

	for _, entry := range s.Environment() {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path = value
		}
	}

	s.SetEnv("PATH=" + directory + string(os.PathListSeparator) + path)
}

func (s *ScenarioRun) omittedPath(relative string) bool {
	for _, dependency := range s.harness.Dependencies {
		if dependency.Recording.SkipsSnapshot(relative) {
			return true
		}
	}

	return false
}
