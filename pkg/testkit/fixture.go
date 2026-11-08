package testkit

import (
	"regexp"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

// Fixture is the Scenario's input tree, optional scripts and declared command expectations.
type Fixture struct {
	// Name identifies the fixture tree.
	Name string
	// Dir locates the fixture directory.
	Dir string
	// Cmd is the exact service invocation requested by the When.
	Cmd string
	// InputPath locates the starting tree overlaid onto the Host project.
	InputPath string
	// ExpectedExitCode is the Then-supplied process status the run's report expects, independent of the exit-code check.
	ExpectedExitCode int
	// StdoutRegexes supplies the run's report with standard-output patterns, independently of CheckStdout assertions.
	StdoutRegexes []*regexp.Regexp
	// JudgeSpec contains the collected semantic outcome clauses.
	JudgeSpec string
	// Stdin supplies input to the service process.
	Stdin []byte
	// Absent names paths removed from the assembled tree before the run.
	Absent []string
	// PrepCmds is seeded at load with nonblank prep.sh text as one command, may be appended to or replaced by callers,
	// and runs once in order before the initial snapshot.
	PrepCmds []string
	// TeardownCmds clean up after all Scenario observations.
	TeardownCmds []string
	// Timeout records the effective suite or Scenario timeout selected for the invocation in the run's report.
	Timeout time.Duration
}

// LoadFixture reads a fixture's input tree and optional preparation and teardown scripts.
func LoadFixture(directory string) (*Fixture, error) {
	loaded, err := runner.LoadFixture(directory)
	if err != nil {
		return nil, err
	}

	return fixtureView(loaded), nil
}

// UseCommand records the exact invocation a When requests, refusing an empty command.
func (f *Fixture) UseCommand(command string) error {
	internal := fixtureToRunner(f)
	err := internal.UseCommand(command)
	f.Cmd = internal.Cmd

	return err
}

func fixtureView(source *runner.Fixture) *Fixture {
	if source == nil {
		return nil
	}

	return &Fixture{Name: source.Name, Dir: source.Dir, Cmd: source.Cmd, InputPath: source.InputPath,
		ExpectedExitCode: source.ExpectedExitCode, StdoutRegexes: source.StdoutRegexes, JudgeSpec: source.JudgeSpec,
		Stdin: source.Stdin, Absent: source.Absent, PrepCmds: source.PrepCmds,
		TeardownCmds: source.TeardownCmds, Timeout: source.Timeout}
}

func fixtureToRunner(source *Fixture) *runner.Fixture {
	if source == nil {
		return nil
	}

	return &runner.Fixture{Name: source.Name, Dir: source.Dir, Cmd: source.Cmd, InputPath: source.InputPath,
		ExpectedExitCode: source.ExpectedExitCode, StdoutRegexes: source.StdoutRegexes, JudgeSpec: source.JudgeSpec,
		Stdin: source.Stdin, Absent: source.Absent, PrepCmds: source.PrepCmds,
		TeardownCmds: source.TeardownCmds, Timeout: source.Timeout}
}
