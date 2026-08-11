package reporter

import (
	"encoding/json"
	"os"
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
	// Source says where the expectations came from, and it is load-bearing
	// for any comparison across runs. SourceSnapshot is what this run was
	// actually held to; SourceRepo is what the source tree says today,
	// which may have moved since. A reader comparing two runs has to be
	// able to tell those apart, or a changed rubric looks like no change
	// at all.
	Source ManifestSource
}

// ManifestSource is where a Manifest's expectations were read from.
type ManifestSource string

const (
	// ManifestAbsent means neither source had anything.
	ManifestAbsent ManifestSource = "absent"
	// ManifestSnapshot is this run's own record of its manifest.
	ManifestSnapshot ManifestSource = "snapshot"
	// ManifestRepo is the fixture folder in the source tree, as it stands
	// now.
	ManifestRepo ManifestSource = "repo"
)

// loadManifest reads one fixture's expectations, preferring this run's
// own snapshot over the source tree.
//
// The snapshot wins because fixture.yaml is never copied into the
// tmpdir: read from the repo, "expected" is always current HEAD, so two
// runs of the same fixture would show identical expectations even when
// the rubric changed between them. A fixture whose folder has since been
// renamed or deleted and which predates the snapshot yields an unloaded
// Manifest rather than an error — the run still happened, and its
// timings are still worth reporting.
func loadManifest(repoRoot, name, dir string) *Manifest {
	manifest := manifestFromSnapshot(dir)
	if manifest != nil {
		return manifest
	}

	fixture, err := runner.LoadFixture(filepath.Join(repoRoot, "tests", "bdd-cli", "fixtures", name))
	if err != nil {
		return &Manifest{Source: ManifestAbsent}
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
		Source:       ManifestRepo,
	}
}

// manifestFromSnapshot reads the manifest this run recorded for itself,
// or nil when it recorded none.
func manifestFromSnapshot(dir string) *Manifest {
	blob, err := os.ReadFile(filepath.Join(dir, runner.SpawnLogDir, runner.ManifestSnapshotFile))
	if err != nil {
		return nil
	}

	var snapshot runner.ManifestSnapshot

	err = json.Unmarshal(blob, &snapshot)
	if err != nil {
		return nil
	}

	return &Manifest{
		Command:      snapshot.Cmd,
		Answers:      snapshot.Answers,
		PrepCmds:     snapshot.PrepCmds,
		TeardownCmds: snapshot.TeardownCmds,
		ExpectedExit: snapshot.ExpectedExit,
		StdoutChecks: snapshot.StdoutRegexes,
		JudgeSpec:    snapshot.JudgeSpec,
		InputPath:    snapshot.InputPath,
		Loaded:       true,
		Source:       ManifestSnapshot,
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
