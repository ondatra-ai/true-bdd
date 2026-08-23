package reporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// Manifest is the fixture's own declaration of what the harness ran: the
// CLI invocation, the shell commands bracketing it, and what the run was
// asserted against. Read through runner.LoadFixture, the shape's authority, rather than re-parsed here.
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
	// Source says where the expectations came from: the snapshot is what
	// this run was held to, the repo is what the source tree says today —
	// conflating them makes a changed rubric look like no change at all.
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

// loadManifest reads one fixture's expectations, preferring this run's own
// snapshot over the source tree.
//
//	snapshot  what this run was actually held to — the repo always shows
//	          current HEAD, which would hide rubric drift across runs
//	repo      timings only; Command/ExpectedExit/etc. stay BLANK rather
//	          than backfilled from today's registry, which would falsely
//	          claim this run was held to a rubric it never saw
//	missing   unloaded Manifest, not an error — timings still count
func loadManifest(repoRoot, name, dir string) *Manifest {
	manifest := manifestFromSnapshot(dir)
	if manifest != nil {
		return manifest
	}

	fixture, err := runner.LoadFixture(filepath.Join(repoRoot, "tests", "bdd-cli", "fixtures", name))
	if err != nil {
		return &Manifest{Source: ManifestAbsent}
	}

	return &Manifest{
		PrepCmds:     fixture.PrepCmds,
		TeardownCmds: fixture.TeardownCmds,
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
