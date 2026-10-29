package testrunner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

func TestTargetAbsentCopyOmitsEveryServiceAndKeepsTestCommands(t *testing.T) {
	t.Parallel()

	host := t.TempDir()

	names := []string{"services/a/main.go", "services/b/main.go", "tmp/old/result",
		".git/config", "tests/cli/run.sh", "docs/scenarios.yaml"}
	for _, name := range names {
		err := disk.Write(filepath.Join(host, name), []byte("fixture"), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := disk.Executable(filepath.Join(host, "tests/cli/run.sh"))
	if err != nil {
		t.Fatal(err)
	}

	copyRoot := filepath.Join(host, "tmp/copy")

	err = testrunner.CopyWithoutTarget(host, copyRoot, []string{"services/a", "./services/b"})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"services/a", "services/b", "tmp", ".git"} {
		_, err := os.Stat(filepath.Join(copyRoot, name))
		if !os.IsNotExist(err) {
			t.Fatalf("omitted %s exists: %v", name, err)
		}
	}

	info, err := os.Stat(filepath.Join(copyRoot, "tests/cli/run.sh"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("executable lost: %v", err)
	}

	_, err = os.Stat(filepath.Join(host, "services/a/main.go"))
	if err != nil {
		t.Fatal("source service changed", err)
	}

	checkCopiedCommandDir(t, host, copyRoot)
}

func checkCopiedCommandDir(t *testing.T, host, copyRoot string) {
	t.Helper()

	original := testrunner.Config{ConfigFile: filepath.Join(host, "tests/cli/config.yaml")}

	cfg, err := testrunner.CopiedConfig(original, host, copyRoot)
	if err != nil {
		t.Fatal(err)
	}

	if testrunner.CommandDir(cfg) != filepath.Join(copyRoot, "tests/cli") {
		t.Fatalf("copy command cwd: %s", testrunner.CommandDir(cfg))
	}
}
