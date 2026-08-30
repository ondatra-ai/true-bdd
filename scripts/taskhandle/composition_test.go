package taskhandle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/taskhandle"
)

// commit and merge are package imports — commit.Embed() and merge.Embed(),
// which are Start(nil).Main(). These two tests are what keeps that true when
// somebody widens an allowlist or rewrites a prompt.

// A blanket Bash(go *) would let a turn run ./scripts/cmd/commit itself.
func TestNoAllowlistCanReachCommitOrMerge(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"Bash(go *)", "Bash(go run *)",
		"scripts/cmd/commit", "scripts/cmd/merge",
	}

	for name, tools := range taskhandle.Allowlists() {
		for _, banned := range forbidden {
			if strings.Contains(tools, banned) {
				t.Errorf("the %s allowlist carries %q — commit and merge are package "+
					"imports, not subprocesses; see commit_step.go", name, banned)
			}
		}
	}
}

// Exactly one turn reaches a skill: step 6's review, because code-review has
// no Go implementation to call instead.
func TestOnlyTheReviewTurnCarriesSkill(t *testing.T) {
	t.Parallel()

	for name, tools := range taskhandle.Allowlists() {
		carries := strings.Contains(tools, "Skill")
		if carries != (name == "review") {
			t.Errorf("the %s allowlist %s Skill; only review may", name,
				map[bool]string{true: "carries", false: "does not carry"}[carries])
		}
	}
}

// A prompt that told a turn to run /pr-commit would route around the import
// without touching a single allowlist.
func TestNoPromptNamesASkillThisPackageReplaces(t *testing.T) {
	t.Parallel()

	forbidden := []string{"pr-commit", "pr-merge", "/task-", "scripts/cmd/commit", "scripts/cmd/merge"}

	prompts, err := filepath.Glob("prompts/*.txt")
	if err != nil || len(prompts) == 0 {
		t.Fatalf("no prompts found: %v", err)
	}

	for _, path := range prompts {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		for _, banned := range forbidden {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s names %q — that workflow is a package import here", path, banned)
			}
		}
	}
}
