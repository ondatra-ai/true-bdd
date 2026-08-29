// Package runner drives BDD-style end-to-end fixtures for true-bdd: each
// fixture is a folder holding the designed project tree under input/; the
// runner overlays it into a tmpdir, execs the binary there, and reports a
// structural diff plus a judge verdict.
//
// What the run DOES — the invocation, the exit code, the stdout, the
// interactive input, the clauses it is judged against — comes from the
// scenario in the registry, not from anything in the folder. A fixture
// carries data; the scenario carries behaviour.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/bash"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/tests/libraries/fstree"
)

const ()

// ErrRepoRootNotFound is returned when FindRepoRoot walks above cwd
// without finding a .git directory.
var ErrRepoRootNotFound = errors.New(
	"BDD runner: repo root (with .git) not found above cwd",
)

// RepoLayer lists subtrees the runner pre-copies from the real repo
// into each fixture's tmpdir BEFORE overlaying the fixture's input
// tree: the live engine ingredients (checklists, templates, config).
func RepoLayer() []string {
	return []string{
		"true-bdd",
		"templates",
	}
}

// NewSessionRoot creates a fresh per-test-invocation directory under
// `<repoRoot>/tmp/test_run/<YYYY-MM-DD_HH-MM-SS>/`; each fixture gets
// its own subdirectory beneath it (see prepareRunDir).
func NewSessionRoot() (string, error) {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}

	stamp := time.Now().Format("2006-01-02_15-04-05")
	root := filepath.Join(repoRoot, "tmp", "test_run", stamp)

	err = disk.Dir(root, disk.Shared)
	if err != nil {
		return "", fmt.Errorf("mkdir session root %s: %w", root, err)
	}

	return root, nil
}

// FindRepoRoot walks up from cwd until it finds a directory containing
// a `.git` entry — the unambiguous repository marker. Exported for
// reuse by the harness fixture materializer.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}

	for {
		_, statErr := os.Stat(filepath.Join(dir, ".git"))
		if statErr == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrRepoRootNotFound
		}

		dir = parent
	}
}

// FileChange describes one file's diff between the fixture's input/ and
// the post-run state of the tmpdir — an alias for fstree.Change so the
// aiproxy shim's per-call diffs and the runner's per-run diffs match.
type FileChange = fstree.Change

// Fixture is one scenario's on-disk data plus the behaviour the
// registry asked of it: directory-derived fields (Name, PrepCmds, ...)
// vs scenario-derived ones (Cmd, ExpectedExitCode, ...), zero until set.
type Fixture struct {
	Name             string
	Dir              string
	Cmd              string // single-line invocation from the scenario's When step
	InputPath        string // path (relative to Dir) of the directory tree overlaid onto the tmpdir
	ExpectedExitCode int    // from the scenario's Then step
	StdoutRegexes    []*regexp.Regexp
	// JudgeSpec is the rubric this run was judged against, rendered by the
	// suite from the scenario's `judge:` clauses — written for the report's
	// expected-vs-actual column, not read by the judge itself (see Clauses).
	JudgeSpec string
	Stdin     []byte   // interactive input, set by the scenario's Given step
	PrepCmds  []string // from prep.sh, run in the tmpdir before the pre-run snapshot
	// TeardownCmds: from teardown.sh, run in the tmpdir after the
	// post-run snapshot whatever the verdict.
	TeardownCmds []string
	// ChecklistPrompts: per-stem prompt selection from
	// checklist-prompts.yaml, applied to the tmpdir checklist during prep.
	ChecklistPrompts map[string][]string
	// Timeout caps the CLI run. Zero means the caller's default; a
	// scenario overrides it with its own `timeout:` key.
	Timeout time.Duration
}

// RunResult bundles everything we observed from one fixture run.
type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Diff     []FileChange
	TmpDir   string // predictable per-fixture path under tmp/test_run/<session>/; preserved after every run
	// StdoutFile/StderrFile are where the CLI's streams were persisted
	// under TmpDir/bdd-cli-logs, empty when capture failed — a failing
	// fixture is read long after the test binary's own output scrolled away.
	StdoutFile string
	StderrFile string
}

// DefaultInputDir is where a fixture's designed project tree lives —
// a convention, not a per-fixture declaration: every fixture has always
// named it the same thing.
const DefaultInputDir = "input"

// PrepScriptFile/TeardownScriptFile live at the fixture ROOT, not
// inside the input tree, and run by content rather than by path: a
// script overlaid into the tmpdir would pollute the diff being graded.
const (
	PrepScriptFile     = "prep.sh"
	TeardownScriptFile = "teardown.sh"
)

// ChecklistPromptsFile narrows which checklist prompts a fixture's run
// walks, keyed by stem:
//
//	us-refine:
//	  - "whether its description field contains a vague word"
const ChecklistPromptsFile = "checklist-prompts.yaml"

// ErrFixtureTreeMissing is returned when a fixture directory has no
// input tree — the one required file, since everything else a fixture
// may ship is optional.
var ErrFixtureTreeMissing = errors.New("fixture has no input tree")

// LoadFixture reads a fixture folder — the DATA half of a scenario. The
// returned Fixture cannot be executed yet: Cmd is empty, since invocation
// is behaviour from the registry — call UseCommand with it before Execute.
func LoadFixture(dir string) (*Fixture, error) {
	// Every file below is optional, so without this check a missing
	// fixture directory would surface as "walk …/input: no such file"
	// deep inside Execute, not a clear error naming the misspelled tree.
	input := filepath.Join(dir, DefaultInputDir)

	info, err := os.Stat(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrFixtureTreeMissing, input, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrFixtureTreeMissing, input)
	}

	fixture := &Fixture{Name: filepath.Base(dir), Dir: dir, InputPath: DefaultInputDir}

	err = loadFixtureScripts(fixture)
	if err != nil {
		return nil, err
	}

	return fixture, nil
}

// loadFixtureScripts reads the optional prep/teardown scripts into the
// commands Execute already knows how to run — each script becomes ONE
// command, so `set -e`/multi-line pipelines behave, not split into shells.
func loadFixtureScripts(fixture *Fixture) error {
	prep, err := readOptionalScript(fixture.Dir, PrepScriptFile)
	if err != nil {
		return err
	}

	teardown, err := readOptionalScript(fixture.Dir, TeardownScriptFile)
	if err != nil {
		return err
	}

	prompts, err := readOptionalChecklistPrompts(fixture.Dir)
	if err != nil {
		return err
	}

	fixture.PrepCmds = prep
	fixture.TeardownCmds = teardown
	fixture.ChecklistPrompts = prompts

	return nil
}

// readOptionalChecklistPrompts reads the fixture's checklist selection,
// or nothing when it ships none. A present-but-empty declaration is
// refused (see ErrFilterDeclaredEmpty), not read as "no filter".
func readOptionalChecklistPrompts(dir string) (map[string][]string, error) {
	path := filepath.Join(dir, ChecklistPromptsFile)

	data, err := disk.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string][]string{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ChecklistPromptsFile, err)
	}

	var prompts map[string][]string

	err = yaml.Unmarshal(data, &prompts)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", ChecklistPromptsFile, err)
	}

	if len(prompts) == 0 {
		return nil, fmt.Errorf("%s: %w", ChecklistPromptsFile, ErrFilterDeclaredEmpty)
	}

	return prompts, nil
}

// readOptionalScript returns the script as a single-element command
// list, or nothing when the fixture ships none. A blank file counts as
// absent too — declared scaffolding that supplies nothing shouldn't pass.
func readOptionalScript(dir, name string) ([]string, error) {
	data, err := disk.Read(filepath.Join(dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}

	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}

	return []string{string(data)}, nil
}

// UseCommand sets the invocation the scenario asked for and re-checks
// everything that depends on it — a method, not a field assignment,
// since checklist-stem validation only becomes possible once Cmd is known.
func (f *Fixture) UseCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return fmt.Errorf("%s: %w", f.Name, ErrCmdRequired)
	}

	f.Cmd = trimmed

	return validateChecklistFilters(f)
}

// Execute runs the fixture: build tmpdir, prep, snapshot, then exec.
// runTimeout's deadline starts at the exec, not Execute's call, so setup
// cost doesn't eat the CLI's budget; extraEnv is appended LAST, winning ties (os/exec).
func Execute(
	ctx context.Context,
	fixture *Fixture,
	binPath, sessionRoot string,
	runTimeout time.Duration,
	extraEnv ...string,
) (*RunResult, error) {
	tmpDir, err := prepareRunDir(fixture, sessionRoot)
	if err != nil {
		return &RunResult{TmpDir: tmpDir}, err
	}

	defer runTeardownCommands(tmpDir, fixture.TeardownCmds)

	err = runPrepCommands(tmpDir, fixture.PrepCmds)
	if err != nil {
		return &RunResult{TmpDir: tmpDir}, err
	}

	before, err := fstree.Snapshot(tmpDir, runSnapshotSkipDirs()...)
	if err != nil {
		return &RunResult{TmpDir: tmpDir}, fmt.Errorf("snapshot pre-run: %w", err)
	}

	args := strings.Fields(fixture.Cmd)

	runCtx, cancelRun := context.WithTimeout(ctx, runTimeout)
	defer cancelRun()

	// Stripped, not blanked: the engine under test must look entirely
	// unlaunched-from-a-session, as claudecli's own child does.
	options := cli.Options{Dir: tmpDir, Env: cli.Inherit().Strip("CLAUDECODE").Set(extraEnv...)}
	if fixture.Stdin != nil {
		options.Stdin = bytes.NewReader(fixture.Stdin)
	}

	finished, runErr := spec.Run(runCtx, append([]string{binPath}, args...), options)

	stdout, stderr := finished.Stdout, finished.Stderr
	exitCode := finished.Code

	after, snapErr := fstree.Snapshot(tmpDir, runSnapshotSkipDirs()...)

	// Persist the CLI's own streams, after the post-run snapshot so the
	// transcript never lands in the diff the judge grades.
	spawn := newSpawnLog(tmpDir)
	stdoutFile := spawn.Write("cli", "stdout", []byte(stdout))
	stderrFile := spawn.Write("cli", "stderr", []byte(stderr))

	if snapErr != nil {
		return &RunResult{
			ExitCode:   exitCode,
			Stdout:     stdout,
			Stderr:     stderr,
			TmpDir:     tmpDir,
			StdoutFile: stdoutFile,
			StderrFile: stderrFile,
		}, fmt.Errorf("snapshot post-run: %w", snapErr)
	}

	res := &RunResult{
		ExitCode:   exitCode,
		Stdout:     stdout,
		Stderr:     stderr,
		Diff:       fstree.Diff(before, after),
		TmpDir:     tmpDir,
		StdoutFile: stdoutFile,
		StderrFile: stderrFile,
	}

	// Surface execution errors that aren't "the process exited non-zero"
	// — those are captured by ExitCode and asserted by checks. shell.Run
	// already draws that line: a non-zero exit is not an error there.
	if runErr != nil {
		return res, fmt.Errorf("exec %s: %w", binPath, runErr)
	}

	return res, nil
}

// prepareRunDir creates the tmpdir, pre-populates it from the repo
// allowlist, and overlays the fixture's input/ on top. Snapshotting
// "before" is the caller's job, done after prep so prep effects don't pollute the diff.
func prepareRunDir(fixture *Fixture, sessionRoot string) (string, error) {
	tmpDir := RunDir(sessionRoot, fixture.Name)

	// Wipe any leftover from a same-second collision (e.g. `go test
	// -run X -count=2` re-entering within one second). MkdirAll alone
	// would mix new content with stale files from the prior run.
	err := disk.RemoveTree(tmpDir)
	if err != nil {
		return "", fmt.Errorf("clean run dir %s: %w", tmpDir, err)
	}

	err = disk.Dir(tmpDir, disk.Shared)
	if err != nil {
		return "", fmt.Errorf("create run dir %s: %w", tmpDir, err)
	}

	repoRoot, err := FindRepoRoot()
	if err != nil {
		return tmpDir, fmt.Errorf("find repo root: %w", err)
	}

	for _, sub := range RepoLayer() {
		err = CopyTree(filepath.Join(repoRoot, sub), filepath.Join(tmpDir, sub))
		if err != nil {
			return tmpDir, fmt.Errorf("pre-populate %s: %w", sub, err)
		}
	}

	err = CopyTree(filepath.Join(fixture.Dir, fixture.InputPath), tmpDir)
	if err != nil {
		return tmpDir, fmt.Errorf("overlay input tree: %w", err)
	}

	err = applyChecklistFilters(fixture, tmpDir)
	if err != nil {
		return tmpDir, err
	}

	return tmpDir, nil
}

// runPrepCommands executes each fixture-provided prep command via
// `bash -c`, teed to console and bdd-cli-logs/. Its own budget, decoupled
// from the run timeout: npm/playwright installs are external, cold-cache work.
func runPrepCommands(tmpDir string, prepCmds []string) error {
	if len(prepCmds) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), prepTimeout)
	defer cancel()

	spawn := newSpawnLog(tmpDir)

	for idx, raw := range prepCmds {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		stdout, stderr, flush := spawn.Tee(spawnLogName("prep", idx))

		result, err := bash.Run(ctx, trimmed, cli.Options{
			Dir:    tmpDir,
			Env:    cli.Inherit().Strip("CLAUDECODE"),
			Output: cli.Streams(stdout, stderr),
		})

		flush()

		if err == nil {
			err = result.Err()
		}

		if err != nil {
			return fmt.Errorf("prep[%d] failed (%q): %w", idx, trimmed, err)
		}
	}

	return nil
}

// prepTimeout caps the *entire* prep phase for one fixture (all prep
// commands share the budget). Generous: a cold `npm install` plus a
// browser download is minutes of pure I/O.
const prepTimeout = 15 * time.Minute

// teardownTimeout caps the *entire* teardown phase (all commands share
// it), decoupled from the run ctx so teardown still fires when the run
// hit ITS timeout — exactly when leftover resources most need cleanup.
const teardownTimeout = 2 * time.Minute

// runTeardownCommands executes each fixture-provided teardown command
// via `bash -c` on a context independent of the run timeout, teed to
// bdd-cli-logs/ — failures are logged, never returned, and can't mask the verdict.
func runTeardownCommands(tmpDir string, teardownCmds []string) {
	if len(teardownCmds) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), teardownTimeout)
	defer cancel()

	spawn := newSpawnLog(tmpDir)

	for idx, raw := range teardownCmds {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		stdout, stderr, flush := spawn.Tee(spawnLogName("teardown", idx))

		result, err := bash.Run(ctx, trimmed, cli.Options{
			Dir:    tmpDir,
			Env:    cli.Inherit().Strip("CLAUDECODE"),
			Output: cli.Streams(stdout, stderr),
		})

		flush()

		if err == nil {
			err = result.Err()
		}

		if err != nil {
			slog.Warn("BDD runner: teardown failed",
				"index", idx, "command", trimmed, "error", err)
		}
	}
}

// CopyTree recursively copies the directory tree rooted at src into
// dst, creating directories as needed and overwriting existing files.
// Exported for reuse by the harness fixture materializer.
func CopyTree(src, dst string) error {
	walkErr := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return fmt.Errorf("filepath rel: %w", relErr)
		}

		target := filepath.Join(dst, rel)

		if entry.IsDir() {
			mkErr := disk.Dir(target, disk.Shared)
			if mkErr != nil {
				return fmt.Errorf("mkdir %s: %w", target, mkErr)
			}

			return nil
		}

		return copyFile(path, target)
	})
	if walkErr != nil {
		return fmt.Errorf("walk %s: %w", src, walkErr)
	}

	return nil
}

func copyFile(src, dst string) error {
	return disk.Copy(dst, src, disk.Shared)
}

// Snapshotting and diffing live in tests/libraries/fstree, shared with
// the aiproxy record/replay shim.

// runSnapshotSkipDirs are subtrees the run diff excludes: dependency
// trees (node_modules) installed by prep.sh, whose symlinks fstree.Snapshot
// refuses by design, and test-runner noise (playwright-report, etc).
func runSnapshotSkipDirs() []string {
	return []string{
		".git", "node_modules", ".next",
		"test-results", "playwright-report",
	}
}
