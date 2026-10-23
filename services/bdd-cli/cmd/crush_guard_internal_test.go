package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

func TestCrushGuardPreservesTrailingSpaceTarget(t *testing.T) {
	if os.Getenv("TRUE_BDD_GUARD_TEST_CHILD") == "1" {
		err := newCrushGuardCmd().Execute()
		if err != nil {
			t.Fatal(err)
		}

		return
	}

	root := t.TempDir()
	outside := t.TempDir()

	target := filepath.Join(root, "leak ")

	err := os.Symlink(outside, target)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("TRUE_BDD_GUARD_TEST_CHILD", "1")
	t.Setenv(ai.CrushPolicyEnvVar, `{"read_root":"`+root+`"}`)
	t.Setenv(ai.CrushToolNameEnvVar, "view")
	t.Setenv(ai.CrushToolFilePathEnvVar, target)
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCrushGuardPreservesTrailingSpaceTarget$")

	output, err := child.CombinedOutput()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != ai.CrushGuardDenyExitCode {
		t.Fatalf("guard allowed escaping link %q: %s, %v", target, output, err)
	}

	if !strings.Contains(string(output), outside) {
		t.Fatalf("denial lost target: %s", output)
	}
}
