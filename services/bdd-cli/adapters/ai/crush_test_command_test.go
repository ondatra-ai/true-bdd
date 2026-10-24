package ai_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const replayBashTool = "bash"
const replaySuiteCommand = "go test ./..."

func TestCrushReplayGrant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	policy := ai.CrushGuardPolicy{ReadRoot: root, TestCommand: replaySuiteCommand, TestSelector: "-run", TestDir: root}
	for _, command := range []string{
		replaySuiteCommand,
		"go test ./... -run '^TestSlow$'",
		"go test ./... -run '$(id); | > *'",
	} {
		allowed, reason := policy.DecideCall(ai.CrushToolCall{Name: replayBashTool, Command: command})
		if !allowed {
			t.Errorf("%q denied: %s", command, reason)
		}
	}

	for _, command := range []string{
		"go test ./...; cat /etc/passwd",
		"X=1 go test ./...",
		"cat /etc/passwd",
		"go test ./... -v",
		`go test ./... -run "TestSlow"`,
		"go test ./... -run ''",
		"go test ./... -run 'a' -v",
		"go  test ./...",
		"go test ./... -run 'a\nb'",
		"go test ./... -run 'a\x00b'"} {
		allowed, reason := policy.DecideCall(ai.CrushToolCall{Name: replayBashTool, Command: command})
		if allowed || !strings.Contains(reason,
			"go test ./... -run '<selection>'") {
			t.Errorf("%q: allowed=%v reason=%s", command, allowed, reason)
		}
	}

	allowed, _ := policy.DecideCall(ai.CrushToolCall{
		Name: replayBashTool, Command: policy.TestCommand, WorkingDir: t.TempDir(),
	})
	if allowed {
		t.Fatal("allowed wrong directory")
	}

	for _, tool := range []string{replayBashTool, "job_output", "job_kill"} {
		call := ai.CrushToolCall{Name: tool, Command: policy.TestCommand}
		if ok, reason := policy.DecideCall(call); !ok {
			t.Errorf("%s: %s", tool, reason)
		}

		without := ai.CrushGuardPolicy{ReadRoot: root}
		if ok, _ := without.DecideCall(call); ok {
			t.Errorf("%s allowed without grant", tool)
		}
	}
}

// TestCrushReplayDirectoryResolvesLinks pins the directory used by the shell.
func TestCrushReplayDirectoryResolvesLinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	child := filepath.Join(outside, "child")

	err := disk.Dir(child, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(child, filepath.Join(root, "link"))
	if err != nil {
		t.Fatal(err)
	}

	policy := ai.CrushGuardPolicy{ReadRoot: root, TestCommand: replaySuiteCommand, TestSelector: "-run", TestDir: root}

	call := ai.CrushToolCall{Name: replayBashTool, Command: policy.TestCommand, WorkingDir: "link/.."}
	if ok, _ := policy.DecideCall(call); ok {
		t.Fatal("cleaned link/.. before resolving it")
	}

	policy.TestDir = child

	call.WorkingDir = "link"
	if ok, why := policy.DecideCall(call); !ok {
		t.Fatalf("same directory through link denied: %s", why)
	}
}
