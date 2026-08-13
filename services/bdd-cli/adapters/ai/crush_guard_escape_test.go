package ai_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

// guardTestDirMode is the permission for scratch dirs these tests build.
const guardTestDirMode = 0o755

// TestCrushGuardDeniesSymlinkEscape is the guard's core promise under
// attack. `crush run` has no permission gate of its own, so this policy
// is the ONLY enforcement: a symlink planted inside a granted root must
// not become a write outside it.
func TestCrushGuardDeniesSymlinkEscape(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	granted := filepath.Join(workDir, "tmp")

	err := os.MkdirAll(granted, guardTestDirMode)
	if err != nil {
		t.Fatalf("create granted root: %v", err)
	}

	// The escape target: a directory the turn must never write to.
	forbidden := filepath.Join(workDir, "secrets")

	err = os.MkdirAll(forbidden, guardTestDirMode)
	if err != nil {
		t.Fatalf("create forbidden dir: %v", err)
	}

	link := filepath.Join(granted, "link")

	err = os.Symlink(forbidden, link)
	if err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	policy := ai.NewCrushGuardPolicy(ai.ExecutionMode{
		AllowedTools: []string{readSpec, writeTmp, globSpec, grepSpec},
	}, workDir)

	// Lexically this is inside the granted root; through the link it is
	// not. The write must be denied.
	target := filepath.Join(link, "stolen.txt")

	allowed, reason := policy.Decide("write", target)
	if allowed {
		t.Errorf("write to %s was ALLOWED — a symlink inside the granted root "+
			"escapes the write gate", target)
	}

	if reason == "" {
		t.Error("a denial must carry a reason")
	}

	// The ordinary case still works.
	if ok, why := policy.Decide("write", filepath.Join(granted, "fine.txt")); !ok {
		t.Errorf("a plain write inside the granted root was denied: %s", why)
	}
}

// TestCrushGuardDeniesSiblingPrefix pins the component boundary. This
// is not a regression test — NewCrushGuardPolicy already appends a
// trailing separator to every root, so a sibling never matched. It
// exists so that dropping that separator, or comparing against a root
// built some other way, fails here instead of silently widening the
// only write gate crush has.
func TestCrushGuardDeniesSiblingPrefix(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	for _, dir := range []string{"tmp", "tmpfoo"} {
		err := os.MkdirAll(filepath.Join(workDir, dir), guardTestDirMode)
		if err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	policy := ai.NewCrushGuardPolicy(ai.ExecutionMode{
		AllowedTools: []string{readSpec, writeTmp, globSpec, grepSpec},
	}, workDir)

	sibling := filepath.Join(workDir, "tmpfoo", "escaped.txt")
	if allowed, _ := policy.Decide("write", sibling); allowed {
		t.Errorf("write to %s was ALLOWED — sibling directory shares the "+
			"root's string prefix", sibling)
	}
}
