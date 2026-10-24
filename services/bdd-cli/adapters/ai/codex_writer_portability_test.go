package ai_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

func TestCodexWriterRequestHashIgnoresToolchainAndAuthenticationLocations(t *testing.T) {
	root := t.TempDir()

	hashes := make([]string, 0, 2)

	for _, location := range []string{"installed", "relocated"} {
		tools := filepath.Join(root, location)
		putCodexFile(t, filepath.Join(tools, "auth.json"), `{"dummy":"credentials"}`)
		goEnv := fmt.Sprintf(`{"GOROOT":"%s/go-root","GOMODCACHE":"%s/mod-cache"}`, tools, tools)
		putCodexFile(t, filepath.Join(tools, "go"), fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\n", goEnv))
		putCodexFile(t, filepath.Join(tools, "codex"), captureCodexRequest)
		t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
		t.Setenv("HOME", tools)
		t.Setenv("CODEX_HOME", tools)

		_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
			WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "writer", CodexWriter: true,
			Model: "gpt-6.1-sol", SystemPrompt: "Implement the service.", UserPrompt: "Run go test ./tests/suite/...",
		})
		if err != nil {
			t.Fatal(err)
		}

		request := readCodexFile(t, filepath.Join(root, "argv.bin")) + "\x00" +
			readCodexFile(t, filepath.Join(root, "stdin.txt"))
		hashes = append(hashes, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(request))))
	}

	if hashes[0] != hashes[1] {
		t.Fatalf("request hashes changed with toolchain/binary/home: %v", hashes)
	}

	t.Logf("request hash under both installations: %s", hashes[0])
}

const captureCodexRequest = `#!/bin/sh
printf '%s\000' "$@" > argv.bin
cat > stdin.txt
while [ "$1" != '-o' ]; do shift; done
printf 'done' > "$2"
`
