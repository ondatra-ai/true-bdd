package steps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// runAt builds the minimum State the containment helpers read: a run whose
// tmpdir is `dir`.
func runAt(dir string) *State {
	return &State{Result: &runner.RunResult{TmpDir: dir}}
}

// A path that simply is not there must come back joined, not refused — the
// caller's own os.Stat says "it does not exist to be left alone", which
// tells a reader more than a containment failure would.
func TestContainedPathAllowsAMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	got, err := runAt(dir).containedPath("docs/never-written.yaml")
	if err != nil {
		t.Fatalf("a missing path inside the run dir must be allowed, got %v", err)
	}

	if want := filepath.Join(dir, "docs/never-written.yaml"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The hole a leaf-only check leaves open: `escape` links out of the tree
// and `escape/missing` does not exist, so nothing about the leaf reveals
// the escape — and the caller's own read would follow the link out.
func TestContainedPathRefusesAMissingLeafUnderAnEscapingSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := t.TempDir()

	err := os.Symlink(outside, filepath.Join(dir, "escape"))
	if err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err = runAt(dir).containedPath("escape/missing.yaml")
	if !errors.Is(err, ErrPathEscapesRun) {
		t.Fatalf("want ErrPathEscapesRun, got %v", err)
	}
}

// The same hole for a directory argument, which takes the other helper.
func TestContainedDirRefusesAMissingPathUnderAnEscapingSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := t.TempDir()

	err := os.Symlink(outside, filepath.Join(dir, "escape"))
	if err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err = runAt(dir).containedDir("escape/steps")
	if !errors.Is(err, ErrPathEscapesRun) {
		t.Fatalf("want ErrPathEscapesRun, got %v", err)
	}
}

// A symlink that stays inside the tree is legitimate and must be followed,
// not refused — the check is about leaving, not about links.
func TestContainedPathFollowsASymlinkThatStaysInside(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.MkdirAll(filepath.Join(dir, "real"), 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir, "real", "story.yaml"), []byte("story: {}\n"), 0o600)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	err = os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link"))
	if err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := runAt(dir).containedPath("link/story.yaml")
	if err != nil {
		t.Fatalf("an inside symlink must be followed, got %v", err)
	}

	if got == "" {
		t.Fatal("expected a resolved path")
	}
}

// Traversal is named as traversal before any filesystem call, so the
// failure says what the scenario got wrong rather than "no such file".
func TestContainedPathRefusesTraversalAndAbsolutePaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for name, rel := range map[string]string{
		"climbing":      "../outside.yaml",
		"deep climbing": "docs/../../outside.yaml",
		"absolute":      "/etc/passwd",
		"empty":         "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := runAt(dir).containedPath(rel)
			if !errors.Is(err, ErrPathEscapesRun) {
				t.Fatalf("want ErrPathEscapesRun for %q, got %v", rel, err)
			}
		})
	}
}
