package bootstrap_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestBuildCodeModelOverrideReachesCodexWriter(t *testing.T) {
	container, directory := prepareWriterOverride(t, "coder=codex:gpt-6.1-sol")
	authentication := filepath.Join(directory, "authentication")
	t.Setenv("CODEX_HOME", authentication)

	script := `#!/bin/sh
printf '%s\n' "$@" > arguments
/bin/cat > request
/bin/cp "$CODEX_HOME/config.toml" writer-config
while [ "$#" -gt 0 ]; do
 if [ "$1" = '-o' ]; then
  shift
  printf 'ok' > "$1"
  exit 0
 fi
 shift
done
exit 1
`
	for path, body := range map[string]string{
		filepath.Join(authentication, "auth.json"): "{}",
		filepath.Join(directory, "codex"):          script,
	} {
		err := disk.Write(path, []byte(body), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := disk.Executable(filepath.Join(directory, "codex"))
	if err != nil {
		t.Fatal(err)
	}

	container.BuildCodeFixApplier.UseWriteRoots([]string{"services/demo"})
	container.BuildCodeFixApplier.UseTestCommand(&provider.TestCommand{
		Command: "go test ./tests/suite", Selector: "-run", Dir: directory,
	})

	_, err = container.BuildCodeFixApplier.Apply(t.Context(), validate.ApplyParams{
		Subject: &commands.BuildCodeItem{}, SubjectID: "failing-set", FixPrompt: "repair",
		TmpDir: container.RunDir.GetTmpOutPath(), ModelTier: "coder",
		ClaudeWriter: true, CodexWriter: true,
	})
	if !errors.Is(err, pkgerrors.ErrFixApplierNoContent) {
		t.Fatalf("writer returned %v", err)
	}

	assertOverriddenCodexWriter(t, directory)
}

func assertOverriddenCodexWriter(t *testing.T, directory string) {
	t.Helper()

	for name, wanted := range map[string][]string{
		"arguments": {"-m\ngpt-6.1-sol\n", "--strict-config\n", "--json\n"},
		"writer-config": {`default_permissions = "writer"`, "[permissions.writer.filesystem]",
			"[permissions.writer.network]\nenabled = false"},
		"request": {"Whole suite: `go test ./tests/suite`", "Selection: `go test ./tests/suite -run '<selection>'`",
			"poll that process with your native process tool until it completes"},
		"tmp/true-bdd.log.json": {`"role":"apply","cli":"codex","model":"gpt-6.1-sol"`},
	} {
		body, err := disk.Read(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}

		for _, want := range wanted {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s missing %q: %s", name, want, body)
			}
		}

		if name == "request" && (strings.Contains(string(body), "job_output") || strings.Contains(string(body), "cd '")) {
			t.Fatal("codex received another writer's command forms")
		}
	}
}
