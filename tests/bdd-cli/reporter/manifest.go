package reporter

import (
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

// Manifest is the fixture's own declaration of what the harness ran:
// the CLI invocation, the shell commands bracketing it, and what the run
// was asserted against.
//
// Read through runner.LoadFixture rather than re-parsed here — the
// runner is the authority on manifest shape, and a reporter with its own
// YAML reader would drift from it silently.
type Manifest struct {
	Command      string
	Answers      string
	PrepCmds     []string
	TeardownCmds []string
	ExpectedExit int
	StdoutChecks []string
	JudgeSpec    string
	InputPath    string
	Loaded       bool
}

// loadManifest reads one fixture's manifest from the source tree. A
// fixture whose folder has since been renamed or deleted yields an
// unloaded Manifest rather than an error: the run still happened, and
// its timings are still worth reporting.
func loadManifest(repoRoot, name string) *Manifest {
	dir := filepath.Join(repoRoot, "tests", "bdd-cli", "fixtures", name)

	fixture, err := runner.LoadFixture(dir)
	if err != nil {
		return &Manifest{}
	}

	checks := make([]string, 0, len(fixture.StdoutRegexes))
	for _, pattern := range fixture.StdoutRegexes {
		checks = append(checks, pattern.String())
	}

	return &Manifest{
		Command:      fixture.Cmd,
		Answers:      string(fixture.Stdin),
		PrepCmds:     fixture.PrepCmds,
		TeardownCmds: fixture.TeardownCmds,
		ExpectedExit: fixture.ExpectedExitCode,
		StdoutChecks: checks,
		JudgeSpec:    fixture.JudgeSpec,
		InputPath:    fixture.InputPath,
		Loaded:       true,
	}
}

// Invocation is the command the harness actually executed: the built
// binary, the manifest's `cmd:` as its arguments, in the fixture's
// tmpdir.
func (m *Manifest) Invocation(tmpDir string) Invocation {
	if !m.Loaded || m.Command == "" {
		return Invocation{}
	}

	return Invocation{
		Binary: "true-bdd",
		Args:   splitCommand(m.Command),
		Dir:    tmpDir,
		Known:  true,
	}
}

// splitCommand splits the manifest's single-line invocation the same way
// the runner does (strings.Fields) before handing it to exec.
func splitCommand(command string) []string {
	return strings.Fields(command)
}
