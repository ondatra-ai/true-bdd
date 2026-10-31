package ai_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestCodexValidationCannotReuseAnEarlierAnswer(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	executable := filepath.Join(bin, "codex")
	putCodexFile(t, executable, `#!/bin/sh
while [ "$1" != '-o' ]; do shift; done
printf '{"steps":{}}' > "$2"
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	request := ai.Request{WorkDir: root, TmpDir: root, Label: "validation", ResultSchema: `{"type":"object"}`}
	client := ai.NewCodexProvider()

	_, err := client.Execute(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	putCodexFile(t, executable, "#!/bin/sh\nexit 0\n")

	_, err = client.Execute(t.Context(), request)
	if !errors.Is(err, pkgerrors.ErrProviderNoOutput) {
		t.Fatalf("missing second answer: got %v", err)
	}
}
