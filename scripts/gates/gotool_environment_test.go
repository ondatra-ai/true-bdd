package gates_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
)

func TestOracleBuildGitDropsRepositoryEnvironment(t *testing.T) {
	preserveCheckout(t)
	root := scratchDirectory(t)
	seedRepository(t, root)
	write(t, root, "go.mod", "module example.com/oracle\n\ngo 1.25\n")
	write(t, root, "services/bdd-cli/main.go", "package main\nfunc main() {}\n")
	local := strings.Fields(gitOutput(t, root, "rev-parse", "--local-env-vars"))
	capture := captureIndirectGit(t, root, local)

	for _, key := range local {
		t.Setenv(key, "hook-value")
	}

	t.Setenv("GIT_AUTHOR_NAME", "Kept author")

	code, err := gotool.BuildOracle(t.Context(), filepath.Join(root, "oracle"),
		gotool.OracleOptions{Directory: root, Output: io.Discard})
	if err != nil || code != 0 {
		t.Fatalf("oracle build: code %d: %v", code, err)
	}

	assertCleanEnvironment(t, capture, local, "GIT_AUTHOR_NAME=Kept author")

	commands, err := os.ReadFile(capture + ".commands")
	if err != nil {
		t.Fatal(err)
	}

	for _, query := range []string{"status --porcelain", "log -1"} {
		if !strings.Contains(string(commands), query) {
			t.Errorf("Go VCS stamping did not run %q: %s", query, commands)
		}
	}
}

func TestGoSubprocessesDropRepositoryEnvironment(t *testing.T) {
	preserveCheckout(t)
	root := scratchDirectory(t)
	local := strings.Fields(gitOutput(t, root, "rev-parse", "--local-env-vars"))
	capture := filepath.Join(root, "go-environment")
	bin := filepath.Join(root, "bin")
	write(t, bin, "go", `#!/bin/sh
env > "$GO_ENV_CAPTURE"
printf '{"GOROOT":"root","GOMODCACHE":"cache","GOCACHE":"build"}'
`)

	err := os.Chmod(filepath.Join(bin, "go"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("GO_ENV_CAPTURE", capture)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, key := range local {
		t.Setenv(key, "hook-value")
	}

	t.Setenv("GIT_AUTHOR_NAME", "Kept author")

	for environmentName, environment := range map[string]cli.Env{
		"inherited": cli.Inherit().Set("GIT_AUTHOR_NAME=Option author"),
		"exact":     cli.Exact(append(os.Environ(), "GIT_AUTHOR_NAME=Option author")),
	} {
		for name, operation := range goOperations(t, root, cli.Options{Env: environment}) {
			t.Run(environmentName+"/"+name, func(t *testing.T) {
				err := operation()
				if err != nil {
					t.Fatal(err)
				}

				kept := "GIT_AUTHOR_NAME=Kept author"

				switch name {
				case "build", "covered", "profile", "bdd":
					kept = "GIT_AUTHOR_NAME=Option author"
				}

				assertCleanEnvironment(t, capture, local, kept)
			})
		}
	}
}

func goOperations(t *testing.T, root string, opt cli.Options) map[string]func() error {
	t.Helper()

	return map[string]func() error{
		"build":   func() error { return gotool.Build(opt, root, "binary", ".") },
		"covered": func() error { return gotool.BuildCovered(opt, root, "binary", ".", "./...") },
		"profile": func() error { return gotool.CoverProfile(opt, root, "profile") },
		"bdd":     func() error { return gotool.ReplayBDD(opt, ".", "binary") },
		"trimmed": func() error { return gotool.BuildTrimmed(t.Context(), root, "binary") },
		"roots": func() error {
			_, err := gotool.ReadRoots(t.Context(), root)

			return err
		},
		"environment": func() error {
			_, err := gotool.Environment(t.Context())

			return err
		},
		"toolchain": func() error {
			_, err := gotool.ToolchainRoots(t.Context())

			return err
		},
		"oracle-build": func() error {
			_, err := gotool.BuildOracle(t.Context(), "binary", gotool.OracleOptions{Directory: root, Output: io.Discard})

			return err
		},
		"oracle-replay": func() error {
			_, err := gotool.ReplayOracle(t.Context(), "binary", gotool.OracleOptions{Directory: root, Output: io.Discard})

			return err
		},
	}
}

func captureIndirectGit(t *testing.T, root string, local []string) string {
	t.Helper()

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	capture := filepath.Join(root, "git-environment")
	bin := filepath.Join(root, "capture-bin")
	write(t, bin, "git", `#!/bin/sh
printf '%s\n' "$*" >> "$GIT_ENV_CAPTURE.commands"
env >> "$GIT_ENV_CAPTURE"
for name in $GIT_ENV_LOCAL; do unset "$name"; done
exec "$GIT_ENV_REAL" "$@"
`)

	err = os.Chmod(filepath.Join(bin, "git"), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_ENV_CAPTURE", capture)
	t.Setenv("GIT_ENV_LOCAL", strings.Join(local, " "))
	t.Setenv("GIT_ENV_REAL", realGit)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	return capture
}
