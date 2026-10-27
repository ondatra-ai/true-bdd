package ai_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/git"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestCodexDeclaredLibrariesCalculatorProbe(t *testing.T) {
	probe := os.Getenv("TRUE_BDD_CODEX_SANDBOX_PROBE")
	if probe == "" {
		t.Skip("set TRUE_BDD_CODEX_SANDBOX_PROBE to the installed codex binary")
	}

	root := t.TempDir()

	fixture, err := filepath.Abs("../../../../tests/true-bdd/fixtures/build-code-spec-scenario/input")
	if err != nil {
		t.Fatal(err)
	}

	err = disk.CopyTree(root, fixture, disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	putCodexFile(t, filepath.Join(root, "services/calc/calc.go"),
		"package calc\nfunc Double(value int) int { return value * 2 }\n")
	prepareCalculatorModules(t, root)

	arch, err := architecture.Load(filepath.Join(root, "docs/architecture/architecture.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	roots, err := arch.DownloadLibraries(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}

	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), libraryProbeScript)
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"),
		Label: "library-probe", CodexWriter: true, Mode: ai.ExecutionMode{LibraryReadRoots: roots,
			SourceWriteRoots: []string{"services/calc"}}})
	log := readCodexFile(t, filepath.Join(root, "tmp/library-probe.log"))
	t.Log(log)

	const expectedDenials = 3
	if strings.Count(log, "Operation not permitted")+strings.Count(log, "Permission denied") != expectedDenials {
		t.Fatal("sandbox did not report the three expected access denials")
	}

	if err != nil {
		t.Fatal(err)
	}
}

func prepareCalculatorModules(t *testing.T, root string) {
	t.Helper()

	const moduleVerb = "mod"
	for _, args := range [][]string{
		{moduleVerb, "edit", "-require=github.com/ondatra-ai/true-bdd@v0.0.0-20261026072958-954797a04424"},
		{moduleVerb, "tidy"},
		{moduleVerb, "download", "github.com/go-chi/chi/v5@v5.2.3"},
	} {
		command := exec.CommandContext(t.Context(), "go", args...)
		command.Dir = root
		command.Env = git.ScrubEnvironment(os.Environ())

		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("go %v: %s: %v", args, output, err)
		}
	}
}

const libraryProbeScript = `#!/bin/sh
set -eu
probe="$TRUE_BDD_CODEX_SANDBOX_PROBE"
: > tmp/library-probe.log
"$probe" sandbox -C "$PWD" -P writer /bin/sh -ec '
 test -z "$(ls -A "$GOCACHE")"
 printf "FRESH GOCACHE: %s\n" "$GOCACHE"
 go test -tags bdd -count=1 ./tests/calc/...
 printf "DENY undeclared github.com/go-chi/chi/v5@v5.2.3\n"
 if cat "$GOMODCACHE/github.com/go-chi/chi/v5@v5.2.3/chi.go"; then exit 61; fi
 printf "DENY undeclared version metadata\n"
 if cat "$GOMODCACHE/cache/download/github.com/go-chi/chi/v5/@v/v5.2.3.mod"; then exit 62; fi
 printf "DENY writes to tests\n"
 if printf forbidden >> tests/calc/calc_test.go; then exit 63; fi
' >> tmp/library-probe.log 2>&1
while [ "$1" != '-o' ]; do shift; done
printf 'probe passed' > "$2"
`
