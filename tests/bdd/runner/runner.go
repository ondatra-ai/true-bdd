// Package runner drives BDD-style end-to-end fixtures for bdd-cli:
// each fixture is a folder with `cmd`, `input/`, and `expected/`; the
// runner copies input into a tmpdir, execs the binary there, and
// reports a structural diff plus a judge verdict.
package runner

import (
	"bufio"
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
	"strconv"
	"strings"
)

const (
	dirPerm  fs.FileMode = 0o755
	filePerm fs.FileMode = 0o644
)

// ErrEmptyCmdFile is returned when a fixture's `cmd` file exists but
// has no executable invocation in it.
var ErrEmptyCmdFile = errors.New("cmd file is empty")

// ErrRepoRootNotFound is returned when findRepoRoot walks above cwd
// without finding a .git directory.
var ErrRepoRootNotFound = errors.New(
	"BDD runner: repo root (with .git) not found above cwd",
)

// repoLayer lists subtrees the runner pre-copies from the real repo
// into each fixture's tmpdir BEFORE overlaying input/. These are the
// live engine ingredients (checklists, prompt templates, engine
// config). Anything outside this list must be provided by the fixture
// itself under input/.
func repoLayer() []string {
	return []string{
		"bdd-cli",
		"templates",
	}
}

// findRepoRoot walks up from cwd until it finds a directory containing
// a `.git` entry. `.git` is used (not `go.mod`) because the bdd-cli
// module lives at scripts/bdd-cli/, so go.mod-based detection would
// stop one level below the repo root.
func findRepoRoot() (string, error) {
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
	ExpectedExitCode int
	StdoutRegexes    []*regexp.Regexp
	JudgeSpec        string // contents of expected/judge.md
	Stdin            []byte // contents of optional `answers` file, fed to subprocess stdin
}

// RunResult bundles everything we observed from one fixture run.
type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Diff     []FileChange
	TmpDir   string // preserved on failure for debugging
}

// LoadFixture parses a fixture folder.
func LoadFixture(dir string) (*Fixture, error) {
	cmdBytes, err := os.ReadFile(filepath.Join(dir, "cmd"))
	if err != nil {
		return nil, fmt.Errorf("read cmd: %w", err)
	}

	cmd := strings.TrimSpace(string(cmdBytes))
	if cmd == "" {
		return nil, ErrEmptyCmdFile
	}

	exitCode, err := readExitCode(filepath.Join(dir, "expected", "exit_code"))
	if err != nil {
		return nil, err
	}

	regexes, err := readStdoutRegexes(filepath.Join(dir, "expected", "stdout.regex"))
	if err != nil {
		return nil, err
	}

	judgeBytes, err := os.ReadFile(filepath.Join(dir, "expected", "judge.md"))
	if err != nil {
		return nil, fmt.Errorf("read judge.md: %w", err)
	}

	stdinBytes, err := readStdin(filepath.Join(dir, "answers"))
	if err != nil {
		return nil, err
	}

	return &Fixture{
		Name:             filepath.Base(dir),
		Dir:              dir,
		Cmd:              cmd,
		ExpectedExitCode: exitCode,
		StdoutRegexes:    regexes,
		JudgeSpec:        string(judgeBytes),
		Stdin:            stdinBytes,
	}, nil
}

func readStdin(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read answers: %w", err)
	}

	return data, nil
}

func readExitCode(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("read exit_code: %w", err)
	}

	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse exit_code: %w", err)
	}

	return code, nil
}

func readStdoutRegexes(path string) ([]*regexp.Regexp, error) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("open stdout.regex: %w", err)
	}

	defer func() { _ = file.Close() }()

	var regexes []*regexp.Regexp

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		re, compileErr := regexp.Compile(line)
		if compileErr != nil {
			return nil, fmt.Errorf("compile regex %q: %w", line, compileErr)
		}

		regexes = append(regexes, re)
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("scan stdout.regex: %w", err)
	}

	return regexes, nil
}

// Execute runs the fixture. Three-step prep:
//  1. Pre-populate the tmpdir from the repo allowlist (repoLayer).
//     These are the live engine ingredients (checklists, templates,
//     config) — pulled from the real repo so a checklist edit
//     propagates to every fixture automatically.
//  2. Overlay the fixture's input/ on top. Files in input/ win, so
//     per-fixture overrides remain possible.
//  3. Snapshot the post-prep state for the diff (so the judge only
//     sees what the run itself did, not the prep).
//
// Then execs binPath in the tmpdir. The tmpdir is preserved on the
// result so the caller can clean it up (or keep it for inspection on
// failure).
func Execute(ctx context.Context, fixture *Fixture, binPath string) (*RunResult, error) {
	tmpDir, before, err := prepareRunDir(fixture)
	if err != nil {
		return &RunResult{TmpDir: tmpDir}, err
	}

	args := strings.Fields(fixture.Cmd)

	cmd := exec.CommandContext(ctx, binPath, args...)
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
	if snapErr != nil {
		return &RunResult{
			ExitCode: exitCode,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			TmpDir:   tmpDir,
		}, fmt.Errorf("snapshot post-run: %w", snapErr)
	}

	res := &RunResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Diff:     computeDiffFromSnapshots(before, after),
		TmpDir:   tmpDir,
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
// allowlist, overlays the fixture's input/, and snapshots the result
// as the "before-run" state for diffing.
func prepareRunDir(fixture *Fixture) (string, map[string][]byte, error) {
	tmpDir, err := os.MkdirTemp("", "bdd-cli-"+fixture.Name+"-")
	if err != nil {
		return "", nil, fmt.Errorf("mkdir tmp: %w", err)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return tmpDir, nil, fmt.Errorf("find repo root: %w", err)
	}

	for _, sub := range repoLayer() {
		err = copyTree(filepath.Join(repoRoot, sub), filepath.Join(tmpDir, sub))
		if err != nil {
			return tmpDir, nil, fmt.Errorf("pre-populate %s: %w", sub, err)
		}
	}

	err = copyTree(filepath.Join(fixture.Dir, "input"), tmpDir)
	if err != nil {
		return tmpDir, nil, fmt.Errorf("overlay input tree: %w", err)
	}

	before, err := snapshotTree(tmpDir)
	if err != nil {
		return tmpDir, nil, fmt.Errorf("snapshot pre-run: %w", err)
	}

	return tmpDir, before, nil
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

func copyTree(src, dst string) error {
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
