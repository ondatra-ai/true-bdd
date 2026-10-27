package gates_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gitcli "github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

const configVerb = "config"

func TestGitSubprocessesDropRepositoryEnvironment(t *testing.T) {
	preserveCheckout(t)
	root := scratchDirectory(t)
	// Ask Git, rather than duplicating its list in the assertion.
	local := strings.Fields(gitOutput(t, root, "rev-parse", "--local-env-vars"))
	capture := filepath.Join(root, "environment")
	bin := filepath.Join(root, "bin")
	write(t, bin, "git", "#!/bin/sh\nenv > \"$GIT_ENV_CAPTURE\"\nprintf 'commit\\n'\n")

	err := os.Chmod(filepath.Join(bin, "git"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_ENV_CAPTURE", capture)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, name := range local {
		t.Setenv(name, "hook-value")
	}

	t.Setenv("GIT_AUTHOR_NAME", "Kept author")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	repository := gitcli.Repository{Dir: root}

	for name, operation := range map[string]func() error{
		"query": func() error {
			_, err := gitcli.TopLevel()

			return err
		},
		"repository": func() error {
			_, err := repository.ResolveCommit(t.Context(), "HEAD")

			return err
		},
		"archive": func() error {
			_, err := repository.Archive(t.Context(), "HEAD")

			return err
		},
		"snapshot": func() error {
			_, err := repository.InitSnapshot(t.Context())

			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := operation()
			if err != nil {
				t.Fatal(err)
			}

			assertCleanEnvironment(t, capture, local, "GIT_AUTHOR_NAME=Kept author", "GIT_TERMINAL_PROMPT=0")
		})
	}
}

func scratchDirectory(t *testing.T) string {
	t.Helper()
	root := gitOutput(t, ".", "rev-parse", "--show-toplevel")

	directory := filepath.Join(root, "tmp")

	err := os.MkdirAll(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	path, err := os.MkdirTemp(directory, "git-environment-") //nolint:usetesting // Repositories stay in ./tmp (#206).
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(path) })
	write(t, path, "go.mod", "module fixture/git-environment\n\ngo 1.25.0\n")

	return path
}

func gitOutput(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = directory
	command.Env = gitcli.ScrubEnvironment(os.Environ())

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}

	return strings.TrimSpace(string(output))
}

func TestGateSubprocessesDropRepositoryEnvironment(t *testing.T) {
	preserveCheckout(t)
	root := scratchDirectory(t)
	local := strings.Fields(gitOutput(t, root, "rev-parse", "--local-env-vars"))
	capture := filepath.Join(root, "environment")
	bin := filepath.Join(root, "bin")
	write(t, bin, "alint", "#!/bin/sh\nenv > \"$GIT_ENV_CAPTURE\"\nprintf '{\"results\":[]}'\n")

	err := os.Chmod(filepath.Join(bin, "alint"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_ENV_CAPTURE", capture)

	for _, key := range local {
		t.Setenv(key, "hook-value")
	}

	t.Setenv("GIT_AUTHOR_NAME", "Kept author")

	for _, gate := range []gates.Gate{
		{Name: "command", Command: []string{"sh", "-c", `env > "$GIT_ENV_CAPTURE"`}},
		gates.All[0],
		{Name: "fixture phase", Run: func() error {
			runner.Teardown(root, []string{`env > "$GIT_ENV_CAPTURE"`})

			return nil
		}},
	} {
		t.Run(gate.Name, func(t *testing.T) {
			err := gates.Run([]gates.Gate{gate})
			if err != nil {
				t.Fatal(err)
			}

			assertCleanEnvironment(t, capture, local, "GIT_AUTHOR_NAME=Kept author")
		})
	}
}

func TestStagedGatesUseHookIndex(t *testing.T) {
	preserveCheckout(t)
	root := scratchDirectory(t)
	checkout := filepath.Join(root, "checkout")
	sentinel := filepath.Join(root, "sentinel")

	seedRepository(t, checkout)
	seedRepository(t, sentinel)
	write(t, checkout, "pkg/change.go", "package change\n")
	gitOutput(t, checkout, "add", "pkg/change.go")

	index := filepath.Join(root, "partial-index")
	gitIndex(t, checkout, index, "read-tree", "HEAD")
	write(t, checkout, "README.md", "partial commit\n")
	gitIndex(t, checkout, index, "add", "README.md")
	binary := buildGates(t, root)
	fakeAlint(t, root)
	before := sentinelState(t, sentinel)
	t.Chdir(checkout)
	t.Setenv("GIT_DIR", filepath.Join(sentinel, ".git"))
	t.Setenv("GIT_WORK_TREE", sentinel)
	t.Setenv("GIT_INDEX_FILE", index)

	paths, err := gitcli.StagedPaths()
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(paths, []string{"README.md"}) {
		t.Errorf("staged paths = %v; want partial index README.md", paths)
	}

	command := exec.CommandContext(t.Context(), binary, "run", "--staged")

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("gates: %s: %v", output, err)
	}

	if !strings.Contains(string(output), "name=Lint") ||
		strings.Contains(string(output), "name=Build") || strings.Contains(string(output), "name=Test") {
		t.Errorf("wrong gates selected from partial index: %s", output)
	}

	if after := sentinelState(t, sentinel); after != before {
		t.Fatal("staged query changed sentinel")
	}
}

func seedRepository(t *testing.T, directory string) {
	t.Helper()

	err := os.MkdirAll(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	gitOutput(t, directory, "init", "--template=", "-b", "main")
	gitOutput(t, directory, configVerb, "user.name", "Sentinel")
	gitOutput(t, directory, configVerb, "user.email", "sentinel@example.com")
	write(t, directory, "README.md", "seed\n")
	gitOutput(t, directory, "add", ".")
	gitOutput(t, directory, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "commit", "-m", "seed")
}

func gitIndex(t *testing.T, directory, index string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = directory

	command.Env = append(gitcli.ScrubEnvironment(os.Environ()), "GIT_INDEX_FILE="+index)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git temporary index: %s: %v", output, err)
	}
}

func buildGates(t *testing.T, root string) string {
	t.Helper()

	binary := filepath.Join(root, "gates")

	command := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../cmd/gates")
	command.Env = gitcli.ScrubEnvironment(os.Environ())

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build gates: %s: %v", output, err)
	}

	return binary
}

func fakeAlint(t *testing.T, root string) {
	t.Helper()

	bin := filepath.Join(root, "bin")
	write(t, bin, "alint", "#!/bin/sh\nprintf '{\"results\":[]}'\n")

	err := os.Chmod(filepath.Join(bin, "alint"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func sentinelState(t *testing.T, directory string) string {
	t.Helper()

	var state strings.Builder
	state.WriteString(gitOutput(t, directory, "show-ref", "--head"))

	for _, name := range []string{"index", configVerb} {
		data, err := os.ReadFile(filepath.Join(directory, ".git", name))
		if err != nil {
			t.Fatal(err)
		}

		state.Write(data)
	}

	return state.String()
}

func preserveCheckout(t *testing.T) {
	t.Helper()

	binary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	root := gitOutput(t, ".", "rev-parse", "--show-toplevel")

	common := gitOutput(t, root, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}

	environment := gitcli.ScrubEnvironment(os.Environ())

	snapshot := func() []string {
		state := make([]string, 0, 4)

		for _, args := range [][]string{
			{"rev-parse", "HEAD"}, {"status", "--porcelain"}, {configVerb, "--local", "--list"},
			{configVerb, "--file", filepath.Join(common, configVerb), "--list"},
		} {
			command := exec.CommandContext(context.Background(), binary, args...)
			command.Dir = root
			command.Env = environment

			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("repository safety check: %s: %v", output, err)
			}

			state = append(state, string(output))
		}

		return state
	}
	before := snapshot()

	t.Cleanup(func() {
		if after := snapshot(); !slices.Equal(after, before) {
			t.Fatalf("STOP: real repository changed: before %q; after %q", before, after)
		}
	})
}

func TestDrillRepositoriesLeaveHookSentinelUnchanged(t *testing.T) {
	preserveCheckout(t)
	root := scratchDirectory(t)
	sentinel := filepath.Join(root, "sentinel")
	seedRepository(t, sentinel)

	snapshot := filepath.Join(root, "snapshot")
	write(t, snapshot, "go.mod", "module example.com/snapshot\n")
	t.Setenv("TMPDIR", root)
	t.Setenv("GIT_DIR", filepath.Join(sentinel, ".git"))
	t.Setenv("GIT_WORK_TREE", sentinel)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(sentinel, ".git", "index"))
	t.Run("snapshot", func(t *testing.T) {
		before := sentinelState(t, sentinel)

		commit, err := (gitcli.Repository{Dir: snapshot}).InitSnapshot(t.Context())
		if after := sentinelState(t, sentinel); after != before {
			t.Fatal("InitSnapshot changed sentinel refs, index or config")
		}

		if err != nil {
			t.Fatal(err)
		}

		if got := gitOutput(t, snapshot, "rev-parse", "HEAD"); got != commit {
			t.Fatalf("snapshot commit %s; want %s", got, commit)
		}
	})
	t.Run("suite", func(t *testing.T) {
		before := sentinelState(t, sentinel)
		local := strings.Fields(gitOutput(t, root, "rev-parse", "--local-env-vars"))
		capture := captureIndirectGit(t, root, local)
		executor := hookTestExecutor(t, root, sentinel)
		command := exec.CommandContext(t.Context(), "go", "test", "-count=1", "-exec", executor, "./scripts/drill/...")
		command.Dir = filepath.Dir(filepath.Dir(root))
		command.Env = gitcli.ScrubEnvironment(os.Environ())
		output, err := command.CombinedOutput()

		if after := sentinelState(t, sentinel); after != before {
			t.Error("drill suite changed sentinel refs, index or config")
		}

		if err != nil {
			t.Fatalf("drill suite: %s: %v", output, err)
		}

		assertCleanEnvironment(t, capture, local)
	})
}

func hookTestExecutor(t *testing.T, root, sentinel string) string {
	t.Helper()
	t.Setenv("GIT_ENV_SENTINEL", sentinel)
	write(t, root, "hook-test", `#!/bin/sh
export GIT_DIR="$GIT_ENV_SENTINEL/.git" GIT_WORK_TREE="$GIT_ENV_SENTINEL" GIT_INDEX_FILE="$GIT_ENV_SENTINEL/.git/index"
exec "$@"
`)

	executor := filepath.Join(root, "hook-test")

	err := os.Chmod(executor, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	return executor
}

func assertCleanEnvironment(t *testing.T, capture string, local []string, kept ...string) {
	t.Helper()

	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}

	entries := strings.Split(string(data), "\n")
	for _, key := range local {
		for _, entry := range entries {
			if strings.HasPrefix(entry, key+"=") {
				t.Errorf("subprocess inherited %s", key)
			}
		}
	}

	for _, entry := range kept {
		if !slices.Contains(entries, entry) {
			t.Errorf("subprocess lost %s", entry)
		}
	}
}
