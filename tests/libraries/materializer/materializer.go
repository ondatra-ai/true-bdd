package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

const dirPerm = 0o755

// ErrTargetNotEmpty is returned when the target directory already
// exists and contains entries — materializing on top of stale state
// would poison the baseline hash.
var ErrTargetNotEmpty = errors.New("materializer: target directory exists and is not empty")

// ErrRepoRootRequired is returned when a `base: engine` fixture is
// materialized without a resolvable repository root.
var ErrRepoRootRequired = errors.New(
	"materializer: base: engine requires a repo root (pass -repo or run inside the repo)")

// ErrRemovePathMissing is returned when a `remove` entry does not
// exist after the base overlay — a typoed path would otherwise
// silently leave the tree complete.
var ErrRemovePathMissing = errors.New(
	"materializer: remove path does not exist after base overlay")

// ErrChecklistStemUnknown is returned when a checklist_prompts stem
// does not resolve to a live checklist file in the materialized tree —
// the harness analogue of the BDD runner's wrong-stem validation
// (harness fixtures have no `cmd` to derive the stem from).
var ErrChecklistStemUnknown = errors.New(
	"materializer: checklist_prompts stem has no live checklist in the materialized tree")

// Options configures one Materialize call.
type Options struct {
	// FixtureDir is the fixture directory containing fixture.yaml.
	FixtureDir string
	// TargetDir is the directory to materialize into. Created if
	// missing; must be empty if it exists.
	TargetDir string
	// RepoRoot supplies the engine layer for `base: engine` fixtures.
	RepoRoot string
}

// Result is the JSON document printed on stdout after a successful
// materialization. See doc.go for the full contract.
type Result struct {
	Fixture  string            `json:"fixture"`
	Target   string            `json:"target"`
	Base     string            `json:"base"`
	Baseline map[string]string `json:"baseline"`
	Teardown []string          `json:"teardown"`
}

// Materialize prepares the fixture tree in the target directory:
// base overlay → remove → input overlay → checklist filtering → prep →
// baseline tree hash. Teardown commands are validated and echoed but
// never executed.
func Materialize(ctx context.Context, opts Options) (*Result, error) {
	manifest, err := LoadManifest(opts.FixtureDir)
	if err != nil {
		return nil, err
	}

	target, err := prepareTarget(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	err = applyBaseLayer(manifest, opts.RepoRoot, target)
	if err != nil {
		return nil, err
	}

	err = applyRemove(manifest, target)
	if err != nil {
		return nil, err
	}

	err = applyInputOverlay(manifest, opts.FixtureDir, target)
	if err != nil {
		return nil, err
	}

	err = applyChecklistFilters(manifest, target)
	if err != nil {
		return nil, err
	}

	err = runPrepCommands(ctx, manifest, target)
	if err != nil {
		return nil, err
	}

	baseline, err := HashTree(target)
	if err != nil {
		return nil, fmt.Errorf("baseline tree hash: %w", err)
	}

	return &Result{
		Fixture:  filepath.Base(mustAbs(opts.FixtureDir)),
		Target:   target,
		Base:     manifest.Base,
		Baseline: baseline,
		Teardown: manifest.Teardown,
	}, nil
}

// prepareTarget resolves the target to an absolute path, creates it if
// missing, and refuses a non-empty existing directory.
func prepareTarget(targetDir string) (string, error) {
	target, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("resolve target %s: %w", targetDir, err)
	}

	err = os.MkdirAll(target, dirPerm)
	if err != nil {
		return "", fmt.Errorf("create target %s: %w", target, err)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return "", fmt.Errorf("read target %s: %w", target, err)
	}

	if len(entries) > 0 {
		return "", fmt.Errorf("%w: %s", ErrTargetNotEmpty, target)
	}

	return target, nil
}

// applyBaseLayer copies the live engine layer (true-bdd/ + templates/)
// into the target for `base: engine`; `base: none` is a no-op.
func applyBaseLayer(manifest *Manifest, repoRoot, target string) error {
	if manifest.Base != BaseEngine {
		return nil
	}

	if repoRoot == "" {
		return ErrRepoRootRequired
	}

	for _, sub := range runner.RepoLayer() {
		src := filepath.Join(repoRoot, sub)

		_, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("%w: missing engine layer %s", ErrRepoRootRequired, src)
		}

		err = runner.CopyTree(src, filepath.Join(target, sub))
		if err != nil {
			return fmt.Errorf("base overlay %s: %w", sub, err)
		}
	}

	return nil
}

// applyRemove deletes each declared path from the target. Every entry
// must exist — a missing path is a fixture bug, not a no-op.
func applyRemove(manifest *Manifest, target string) error {
	for _, raw := range manifest.Remove {
		cleaned := filepath.Clean(raw)
		path := filepath.Join(target, cleaned)

		_, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrRemovePathMissing, cleaned)
		}

		err = os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("remove %s: %w", cleaned, err)
		}
	}

	return nil
}

func applyInputOverlay(manifest *Manifest, fixtureDir, target string) error {
	if manifest.Input == "" {
		return nil
	}

	err := runner.CopyTree(filepath.Join(fixtureDir, manifest.Input), target)
	if err != nil {
		return fmt.Errorf("input overlay: %w", err)
	}

	return nil
}

// applyChecklistFilters rewrites each declared checklist in the target
// to the selected prompts, reusing the BDD runner's filter verbatim so
// snippet semantics (whitespace-collapsed substring, exactly one live
// non-skipped prompt, no duplicate resolution) stay identical.
func applyChecklistFilters(manifest *Manifest, target string) error {
	for stem, snippets := range manifest.ChecklistPrompts {
		path := filepath.Join(target, "true-bdd", "checklists", stem+".yaml")

		_, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%w: stem %s (%s)", ErrChecklistStemUnknown, stem, path)
		}

		err = runner.FilterChecklistFile(path, snippets)
		if err != nil {
			return fmt.Errorf("filtering %s: %w", path, err)
		}
	}

	return nil
}

// runPrepCommands executes each prep command in the target via
// `bash -c`. Output goes to STDERR — stdout is reserved for the
// result JSON. A non-zero exit aborts the materialization.
func runPrepCommands(ctx context.Context, manifest *Manifest, target string) error {
	for idx, command := range manifest.Prep {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Dir = target
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("prep[%d] failed (%q): %w", idx, command, err)
		}
	}

	return nil
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}
