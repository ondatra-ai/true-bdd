// Package testkit owns the BDD harness: suite bring-up, Scenario runs,
// AI CLI modes, recordings and grading. Suite wiring supplies definitions
// and its State constructor; definitions use ScenarioRun.
package testkit

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// Flags are the harness options registered on a test binary's flag set.
type Flags struct {
	Mode      string
	EngineBin string
}

// RegisterFlags keeps the flags shared by go test, coverage and drill callers.
func RegisterFlags(set *flag.FlagSet) *Flags {
	options := &Flags{}
	set.StringVar(&options.Mode, "mode", "", "per-caller AI CLI mode, e.g. services:replay,tests:live")
	set.StringVar(&options.EngineBin, "engine-bin", "",
		"absolute path of a prebuilt service binary to drive instead of building one")

	return options
}

// Modes parses explicit caller modes; absence stays legal for coverage guards.
func (f *Flags) Modes() (runner.Modes, error) {
	modes, err := runner.ParseModes(f.Mode)
	if err != nil {
		return runner.Modes{}, fmt.Errorf("parse -mode: %w", err)
	}

	return modes, nil
}

// ErrModeRequired marks a Scenario requested without explicit caller modes.
var ErrModeRequired = errors.New("-mode is required to run a scenario")

// RequireMode refuses a Scenario boot; guards need only Modes.
func (f *Flags) RequireMode() error {
	modes, err := f.Modes()
	if err != nil {
		return err
	}

	if !modes.Set() {
		return fmt.Errorf("%w; want %s", ErrModeRequired, runner.ModeSpecForm())
	}

	return nil
}

// ErrEngineBinRelative marks an override go test would resolve from the suite directory.
var ErrEngineBinRelative = errors.New("-engine-bin must be an absolute path")

// PrebuiltEngine validates the override lazily, at the first step needing the service.
func (f *Flags) PrebuiltEngine() (string, error) {
	if !filepath.IsAbs(f.EngineBin) {
		return "", fmt.Errorf("%w: %s", ErrEngineBinRelative, f.EngineBin)
	}

	_, err := os.Stat(f.EngineBin)
	if err != nil {
		return "", fmt.Errorf("-engine-bin: %w", err)
	}

	return f.EngineBin, nil
}
