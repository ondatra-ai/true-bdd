package ai_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

// Set TRUE_BDD_CODEX_SANDBOX_PROBE to a real codex binary to exercise its OS sandbox without a model turn.
func TestCodexWriterSequentialTurnsProtectEarlierHomesAndVCS(t *testing.T) {
	paths, err := gotool.Environment(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	putCodexFile(t, filepath.Join(root, "original/auth.json"), `{"dummy":"credentials"}`)
	t.Setenv("CODEX_HOME", filepath.Join(root, "original"))
	bin := filepath.Join(root, "bin")
	putCodexFile(t, filepath.Join(bin, "codex"), sequentialCodexScript)
	cache := filepath.Join(root, "tmp/module-cache")
	putCodexFile(t, filepath.Join(cache, "cache/vcs/dummy.git/objects/marker"), "dummy source")
	goEnv := `{"GOROOT":` + strconv.Quote(paths["GOROOT"]) + `,"GOMODCACHE":` + strconv.Quote(cache) + `}`
	putCodexFile(t, filepath.Join(bin, "go"), fmt.Sprintf(dummyGoEnvironment, goEnv, cache))
	putCodexFile(t, filepath.Join(root, "go.mod"), "module probe\n\ngo 1.26\n")
	putCodexFile(t, filepath.Join(root, "tests/suite/probe_test.go"),
		"package probe\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	for turn := range 2 {
		t.Setenv("TRUE_BDD_TEST_TURN", strconv.Itoa(turn))

		_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
			WorkDir: root, TmpDir: filepath.Join(root, "tmp", fmt.Sprintf("run%d", turn)),
			Label: fmt.Sprintf("writer%d", turn), CodexWriter: true,
		})
		if (err != nil) != (turn == 1) {
			t.Fatalf("turn %d: %v", turn, err)
		}

		home := strings.TrimSpace(readCodexFile(t, filepath.Join(root, "current-home.txt")))
		assertCodexCredentialsRemoved(t, home)

		config := readCodexFile(t, filepath.Join(home, "config.toml"))
		for _, deny := range []string{filepath.Join(root, "tmp/codex-homes"), "/cache/vcs"} {
			if !strings.Contains(config, deny+`" = "deny"`) {
				t.Fatalf("missing denial %s", deny)
			}
		}

		putCodexFile(t, filepath.Join(root, "previous-home.txt"), home)
	}

	if os.Getenv("TRUE_BDD_CODEX_SANDBOX_PROBE") != "" {
		t.Log(readCodexFile(t, filepath.Join(root, "probe.log")))

		if readCodexFile(t, filepath.Join(root, "probe-passed.txt")) != "passed" {
			t.Fatal("live probe did not finish")
		}
	}
}

const sequentialCodexScript = `#!/bin/sh
set -eu
printf '%s' "$CODEX_HOME" > current-home.txt
printf 'dummy marker' > "$CODEX_HOME/marker.txt"
if [ -n "${TRUE_BDD_CODEX_SANDBOX_PROBE:-}" ] && [ -f previous-home.txt ]; then
 previous=$(cat previous-home.txt)
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/cat "$previous/marker.txt" > probe.log 2>&1 && exit 21
 grep 'Operation not permitted' probe.log || exit 22
 cache=$(go env GOMODCACHE)
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/ls "$cache/cache/vcs" >> probe.log 2>&1 && exit 23
 grep 'cache/vcs: Operation not permitted' probe.log || exit 24
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/sh -c \
  "go test -count=1 ./tests/suite/... -run '^TestA$'" >> probe.log 2>&1 || exit 25
 printf passed > probe-passed.txt
fi
while [ "$1" != '-o' ]; do shift; done
printf 'done' > "$2"
if [ "$TRUE_BDD_TEST_TURN" = '1' ]; then exit 7; fi
`

const dummyGoEnvironment = `#!/bin/sh
if [ "$2" = '-json' ]; then
 printf '%%s' '%s'
else
 printf '%%s' '%s'
fi
`
