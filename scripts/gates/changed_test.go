package gates_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()

	path := filepath.Join(dir, name)

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}

	err = os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// repoOnMain is a checkout sitting on main with one commit, which is where
// task-handle calls Changed from: scripts/commit has not cut the branch yet.
func repoOnMain(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	git(t, dir, "init", "--initial-branch=main")
	git(t, dir, "config", "user.email", "t@example.com")
	git(t, dir, "config", "user.name", "t")
	write(t, dir, "README.md", "seed\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "seed")

	return dir
}

func changedIn(t *testing.T, dir string) []string {
	t.Helper()

	t.Chdir(dir)

	paths, err := gates.Changed("main")
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}

	return paths
}

// The regression this test exists for: with base...HEAD, a checkout on main
// with the work still uncommitted reports NOTHING, the fail-safe runs
// everything, and the selector is inert at its only real call site.
func TestChangedSeesUncommittedWorkOnMain(t *testing.T) {
	dir := repoOnMain(t)
	write(t, dir, "README.md", "seed\nedited\n")

	if got := changedIn(t, dir); !slices.Contains(got, "README.md") {
		t.Fatalf("Changed() = %v, want it to include the edited README.md", got)
	}
}

// A Ticket that adds a file is the ordinary case, and an untracked file is
// invisible to git diff no matter which dots it is given.
func TestChangedSeesUntrackedFiles(t *testing.T) {
	dir := repoOnMain(t)
	write(t, dir, "docs/for_further/new.md", "new\n")

	if got := changedIn(t, dir); !slices.Contains(got, "docs/for_further/new.md") {
		t.Fatalf("Changed() = %v, want it to include the untracked file", got)
	}
}

// End to end: a documentation-only change must select the two cheap gates and
// nothing else. That gap — ~2s against ~140s — is the whole point of §6.
func TestDocumentationChangeSelectsOnlyLint(t *testing.T) {
	dir := repoOnMain(t)
	write(t, dir, "docs/for_further/new.md", "new\n")
	write(t, dir, "README.md", "seed\nedited\n")

	got := names(gates.Select(changedIn(t, dir)))
	want := []string{lintGate}

	if !slices.Equal(got, want) {
		t.Fatalf("selected %v, want %v", got, want)
	}
}
