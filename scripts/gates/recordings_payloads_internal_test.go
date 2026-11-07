package gates

import (
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordingPayloadsMustBeInTheIndex(t *testing.T) {
	t.Chdir(t.TempDir())

	run := func(args ...string) {
		t.Helper()

		command := exec.CommandContext(t.Context(), "git", args...)
		command.Env = git.ScrubEnvironment(os.Environ())

		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, output, err)
		}
	}
	run("init", "-q")

	const (
		cassette = "tests/true-bdd/fixtures/sample/cassettes/codex-001"
		payload  = cassette + "/fsdiff/after/tmp/.DS_Store"
		meta     = cassette + "/meta.json"
	)

	err := os.MkdirAll(filepath.Dir(payload), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	for path, content := range map[string]string{
		".gitignore": ".DS_Store\n",
		meta: `{"fsdiff":{"created":["tmp/.DS_Store"],` +
			`"modified":["changed"],"deleted":["gone"]}}`,
		payload:                            "ignored but on disk",
		cassette + "/fsdiff/after/changed": "changed",
	} {
		err := os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	run("add", ".")

	err = ScanRecordings()
	if err == nil || !strings.Contains(err.Error(), payload) {
		t.Fatalf("ignored on-disk payload passed: %v", err)
	}

	run("add", "-f", payload)

	err = ScanRecordings()
	if err != nil {
		t.Fatalf("staged payload failed: %v", err)
	}

	run("rm", "--cached", "-f", payload)

	err = ScanRecordings()
	if err == nil {
		t.Fatal("staged removal passed while payload remains on disk")
	}
}

func TestRecordingPayloadsUseHookIndex(t *testing.T) {
	t.Chdir(t.TempDir())
	runRecordingGit(t, "", "init", "-q")

	const (
		cassette = "tests/true-bdd/fixtures/sample/cassettes/codex-001"
		payload  = cassette + "/fsdiff/after/created"
	)

	err := os.MkdirAll(filepath.Dir(payload), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	for path, content := range map[string]string{
		cassette + "/meta.json": `{"fsdiff":{"created":["created"]}}`,
		payload:                 "payload",
	} {
		err := os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	index, err := filepath.Abs("partial-index")
	if err != nil {
		t.Fatal(err)
	}

	runRecordingGit(t, index, "add", cassette)
	t.Setenv("GIT_INDEX_FILE", index)

	err = ScanRecordings()
	if err != nil {
		t.Fatalf("payload added only to hook index failed: %v", err)
	}

	runRecordingGit(t, "", "add", cassette)
	runRecordingGit(t, index, "rm", "--cached", payload)

	err = ScanRecordings()
	if err == nil || !strings.Contains(err.Error(), payload) {
		t.Fatalf("payload removed only from hook index passed: %v", err)
	}
}

func runRecordingGit(t *testing.T, index string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)

	var indexEnvironment []string
	if index != "" {
		indexEnvironment = []string{"GIT_INDEX_FILE=" + index}
	}

	command.Env = append(git.ScrubEnvironment(os.Environ()), indexEnvironment...)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}
}
