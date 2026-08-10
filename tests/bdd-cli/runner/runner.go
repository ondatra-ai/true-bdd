// Package runner drives BDD-style end-to-end fixtures for true-bdd:
// each fixture is a folder with `fixture.yaml` (manifest) and the
// referenced input directory; the runner overlays input into a tmpdir,
// execs the binary there, and reports a structural diff plus a judge
// verdict.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	dirPerm  fs.FileMode = 0o755
	filePerm fs.FileMode = 0o644
)

// ErrRepoRootNotFound is returned when FindRepoRoot walks above cwd
// without finding a .git directory.
var ErrRepoRootNotFound = errors.New(
	"BDD runner: repo root (with .git) not found above cwd",
)

// RepoLayer lists subtrees the runner pre-copies from the real repo
// into each fixture's tmpdir BEFORE overlaying the fixture's input
// tree. These are the live engine ingredients (checklists, prompt
// templates, engine config). Anything outside this list must be
// provided by the fixture itself under its input directory.
//
// Exported because the harness fixture materializer
// (tests/materializer) shares the same engine-layer
// convention for its `base: engine` fixtures.
func RepoLayer() []string {
	return []string{
		"true-bdd",
		"templates",
	}
}

// NewSessionRoot creates a fresh per-test-invocation directory under
// `<repoRoot>/tmp/test_run/<YYYY-MM-DD_HH-MM-SS>/`. Each fixture
// in the same `go test` run will get its own subdirectory beneath this
// path (see prepareRunDir), so all fixtures from one invocation are
// grouped together. The repo's `/tmp/` .gitignore rule covers this
// tree, so it is never accidentally committed.
func NewSessionRoot() (string, error) {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}

	stamp := time.Now().Format("2006-01-02_15-04-05")
	root := filepath.Join(repoRoot, "tmp", "test_run", stamp)

	err = os.MkdirAll(root, dirPerm)
	if err != nil {
		return "", fmt.Errorf("mkdir session root %s: %w", root, err)
	}

	return root, nil
}

// FindRepoRoot walks up from cwd until it finds a directory containing
// a `.git` entry — the unambiguous repository marker. (Preferring
// `.git` over `go.mod` is a holdover from the monorepo era when the
// module lived below the repo root; today go.mod sits at the root
// too, so either marker would work.)
//
// Exported for reuse by the harness fixture materializer.
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
// the post-run state of the tmpdir.
type FileChange struct {
	Path   string // path relative to tmpdir
	Kind   string // "created", "modified", "deleted"
	Before []byte // empty for "created"
	After  []byte // empty for "deleted"
}

// Fixture represents one scenario folder loaded from disk.
type Fixture struct {
	Name             string
	Dir              string
	Cmd              string // single-line invocation, e.g. "us create 99.1"
	InputPath        string // path (relative to Dir) of the directory tree overlaid onto the tmpdir
	ExpectedExitCode int
	StdoutRegexes    []*regexp.Regexp
	JudgeSpec        string   // judge rubric from fixture.yaml
	Stdin            []byte   // contents of optional `answers:` field, fed to subprocess stdin
	PrepCmds         []string // optional `prep:` shell commands, run in tmpdir before snapshot
	// TeardownCmds: optional `teardown:` shell commands run in tmpdir
	// after the post-run snapshot. See FixtureManifest.Teardown.
	TeardownCmds []string
	// ChecklistPrompts: optional per-stem prompt selection applied to
	// the tmpdir checklist during prep. See
	// FixtureManifest.ChecklistPrompts.
	ChecklistPrompts map[string][]string
	// Timeout caps the CLI run for this fixture. Zero means the caller's
	// default. See FixtureManifest.Timeout.
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
	// under TmpDir/bdd-cli-logs. Empty when capture failed. A failing
	// fixture is read long after the test binary's own output has
	// scrolled away, so the transcript has to outlive the process.
	StdoutFile string
	StderrFile string
}

// LoadFixture parses a fixture folder.
func LoadFixture(dir string) (*Fixture, error) {
	manifest, regexes, err := LoadFixtureManifest(filepath.Join(dir, "fixture.yaml"))
	if err != nil {
		return nil, err
	}

	var stdinBytes []byte
	if manifest.Answers != "" {
		stdinBytes = []byte(manifest.Answers)
	}

	timeout, err := parseFixtureTimeout(manifest.Timeout)
	if err != nil {
		return nil, err
	}

	fixture := &Fixture{
		Name:             filepath.Base(dir),
		Dir:              dir,
		Cmd:              strings.TrimSpace(manifest.Cmd),
		InputPath:        strings.TrimSpace(manifest.Input),
		ExpectedExitCode: manifest.Expected.ExitCode,
		StdoutRegexes:    regexes,
		JudgeSpec:        manifest.Expected.Judge,
		Stdin:            stdinBytes,
		PrepCmds:         manifest.Prep,
		TeardownCmds:     manifest.Teardown,
		ChecklistPrompts: manifest.ChecklistPrompts,
		Timeout:          timeout,
	}

	err = validateChecklistFilters(fixture)
	if err != nil {
		return nil, err
	}

	return fixture, nil
}

// Execute runs the fixture. Four-step prep:
//  1. Pre-populate the tmpdir from the repo allowlist (RepoLayer).
//     These are the live engine ingredients (checklists, templates,
//     config) — pulled from the real repo so a checklist edit
//     propagates to every fixture automatically.
//  2. Overlay the fixture's input/ on top. Files in input/ win, so
//     per-fixture overrides remain possible.
//  3. Run the fixture's `prep:` shell commands (npm install, etc.)
//     against the tmpdir. Side effects are captured by the pre-run
//     snapshot so they don't pollute the diff handed to the judge.
//  4. Snapshot the post-prep state for the diff (so the judge only
//     sees what the run itself did, not the prep).
//
// Then execs binPath in the tmpdir. The tmpdir lives at
// `<sessionRoot>/<fixture.Name>/`, is preserved on the result, and is
// never auto-cleaned — callers can inspect or `docker compose up` inside
// it after the test exits.
//
// After the run, fixture-declared `teardown:` commands run via a
// deferred call against a fresh context, so long-lived external
// resources (e.g. docker-compose stacks the CLI brought up) get torn
// down even when the CLI itself failed or hit the fixture timeout.
//
// runTimeout caps the CLI invocation ALONE. Its deadline starts
// immediately before the exec, not when Execute was called, so the
// tmpdir build and the pre-run snapshot cannot eat into it — on the
// playwright fixture that snapshot reads the whole tree, node_modules
// included, and charging it to the CLI would fail a run that behaved.
func Execute(
	ctx context.Context,
	fixture *Fixture,
	binPath, sessionRoot string,
	runTimeout time.Duration,
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

	before, err := snapshotTree(tmpDir)
	if err != nil {
		return &RunResult{TmpDir: tmpDir}, fmt.Errorf("snapshot pre-run: %w", err)
	}

	args := strings.Fields(fixture.Cmd)

	runCtx, cancelRun := context.WithTimeout(ctx, runTimeout)
	defer cancelRun()

	cmd := exec.CommandContext(runCtx, binPath, args...)
	cmd.Dir = tmpDir
	cmd.Env = envWithoutClaudeCode(os.Environ())

	if fixture.Stdin != nil {
		cmd.Stdin = bytes.NewReader(fixture.Stdin)
	}

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := cmd.ProcessState.ExitCode()

	after, snapErr := snapshotTree(tmpDir)

	// Persist the CLI's own streams, after the post-run snapshot so the
	// transcript never lands in the diff the judge grades.
	spawn := newSpawnLog(tmpDir)
	stdoutFile := spawn.Write("cli", "stdout", stdout.Bytes())
	stderrFile := spawn.Write("cli", "stderr", stderr.Bytes())

	if snapErr != nil {
		return &RunResult{
			ExitCode:   exitCode,
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			TmpDir:     tmpDir,
			StdoutFile: stdoutFile,
			StderrFile: stderrFile,
		}, fmt.Errorf("snapshot post-run: %w", snapErr)
	}

	res := &RunResult{
		ExitCode:   exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		Diff:       computeDiffFromSnapshots(before, after),
		TmpDir:     tmpDir,
		StdoutFile: stdoutFile,
		StderrFile: stderrFile,
	}

	// Surface execution errors that aren't "the process exited non-zero"
	// — those are captured by ExitCode and asserted by checks.
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return res, fmt.Errorf("exec %s: %w", binPath, runErr)
	}

	return res, nil
}

// prepareRunDir creates the tmpdir, pre-populates it from the repo
// allowlist, and overlays the fixture's input/ on top. Snapshotting
// the "before" state is the caller's responsibility — Execute does it
// after prep commands have a chance to mutate the tree, so prep side
// effects don't pollute the diff.
func prepareRunDir(fixture *Fixture, sessionRoot string) (string, error) {
	tmpDir := RunDir(sessionRoot, fixture.Name)

	// Wipe any leftover from a same-second collision (e.g. `go test
	// -run X -count=2` re-entering within one second). MkdirAll alone
	// would mix new content with stale files from the prior run.
	err := os.RemoveAll(tmpDir)
	if err != nil {
		return "", fmt.Errorf("clean run dir %s: %w", tmpDir, err)
	}

	err = os.MkdirAll(tmpDir, dirPerm)
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

// runPrepCommands executes each fixture-provided prep command in the
// tmpdir via `bash -c`. Stdin is unset; stdout and stderr are teed —
// streamed to the calling `go test -v` output so progress stays visible
// during long installs, and captured under bdd-cli-logs/ so a failed
// prep can still be read afterwards. Any non-zero exit aborts the
// fixture.
//
// The whole prep phase gets its own budget, decoupled from the run ctx:
// `npm install` and `playwright install` are external work that says
// nothing about whether the CLI under test behaves, so charging them to
// the fixture's run timeout would fail a correct fixture on a cold
// cache.
//
// These writes land before the pre-run snapshot, so they appear in both
// snapshots and never show up in the diff.
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

		cmd := exec.CommandContext(ctx, "bash", "-c", trimmed)
		cmd.Dir = tmpDir
		cmd.Env = envWithoutClaudeCode(os.Environ())
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()

		flush()

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

// teardownTimeout caps the *entire* teardown phase for one fixture
// (all teardown commands share the budget). Decoupled from the
// fixture's run ctx so teardown still fires when the run hit its
// timeout — that is exactly when leftover resources are most likely
// to be holding ports / state for the next run.
const teardownTimeout = 2 * time.Minute

// runTeardownCommands executes each fixture-provided teardown command
// in the tmpdir via `bash -c`, against a fresh context independent of
// the run timeout. Stdout/stderr are teed — streamed to the calling
// `go test -v` and captured under bdd-cli-logs/, which is where the
// evidence for a stack that refused to come down has to live, since
// teardown output arrives after the verdict is already decided.
// Failures are logged but never returned — teardown is best-effort
// hygiene and must not mask the primary run verdict.
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

		cmd := exec.CommandContext(ctx, "bash", "-c", trimmed)
		cmd.Dir = tmpDir
		cmd.Env = envWithoutClaudeCode(os.Environ())
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()

		flush()

		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"BDD runner: teardown[%d] failed (%q): %v\n",
				idx, trimmed, err,
			)
		}
	}
}

func envWithoutClaudeCode(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "CLAUDECODE=") {
			continue
		}

		out = append(out, kv)
	}

	return out
}

// CopyTree recursively copies the directory tree rooted at src into
// dst, creating directories as needed and overwriting existing files.
// Exported for reuse by the harness fixture materializer, which layers
// engine base + fixture input with the same overlay semantics.
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
			mkErr := os.MkdirAll(target, dirPerm)
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
	err := os.MkdirAll(filepath.Dir(dst), dirPerm)
	if err != nil {
		return fmt.Errorf("mkdir parent of %s: %w", dst, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}

	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("open dst %s: %w", dst, err)
	}

	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	return nil
}

// computeDiffFromSnapshots compares two filesystem snapshots
// (path → content) and returns the list of created/modified/deleted
// entries. Callers are responsible for taking the snapshots — this
// lets Execute() snapshot the tmpdir at the right moments (after
// prep, after run) without an extra round of file IO.
func computeDiffFromSnapshots(before, after map[string][]byte) []FileChange {
	var changes []FileChange

	for path, beforeBytes := range before {
		afterBytes, present := after[path]
		if !present {
			changes = append(changes, FileChange{
				Path: path, Kind: "deleted", Before: beforeBytes,
			})

			continue
		}

		if !bytes.Equal(beforeBytes, afterBytes) {
			changes = append(changes, FileChange{
				Path: path, Kind: "modified", Before: beforeBytes, After: afterBytes,
			})
		}
	}

	for path, afterBytes := range after {
		if _, present := before[path]; !present {
			changes = append(changes, FileChange{
				Path: path, Kind: "created", After: afterBytes,
			})
		}
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })

	return changes
}

func snapshotTree(root string) (map[string][]byte, error) {
	out := make(map[string][]byte)

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("filepath rel: %w", relErr)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		out[rel] = data

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}

	return out, nil
}
