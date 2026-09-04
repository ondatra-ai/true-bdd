package runner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeConfig lays down an engine config carrying the given models block.
func writeConfig(t *testing.T, models string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "true-bdd.yaml")

	err := os.WriteFile(path, []byte("engine:\n  models:\n"+models), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func TestJudgeModelReadsTheQuickTier(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `    xhigh: "claude:claude-fable-5"
    quick: "claude:claude-sonnet-5"
`)

	model, err := JudgeModel(path)
	if err != nil {
		t.Fatalf("JudgeModel: %v", err)
	}

	if model != "claude-sonnet-5" {
		t.Errorf("JudgeModel = %q, want the pinned sonnet id", model)
	}
}

// No quick tier is a refusal, not a fallback: a judge that silently
// picks its own model is how the verdict substrate drifted before.
func TestJudgeModelRefusesWithoutAClaudeQuickTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		models string
	}{
		{"no quick tier", "    high: \"claude:claude-opus-4-8\"\n"},
		{"quick bound to another cli", "    quick: \"crush:zhipu-coding/glm-5.2\"\n"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := JudgeModel(writeConfig(t, testCase.models))
			if !errors.Is(err, ErrJudgeTierMissing) {
				t.Fatalf("JudgeModel error = %v, want %v", err, ErrJudgeTierMissing)
			}
		})
	}
}

// The repo's own config must resolve — this is the file the suite reads
// at boot, so a typo in it is a suite that cannot grade anything.
func TestJudgeModelResolvesTheRepoConfig(t *testing.T) {
	t.Parallel()

	model, err := JudgeModel(filepath.Join("..", "..", "..", "true-bdd", "true-bdd.yaml"))
	if err != nil {
		t.Fatalf("JudgeModel on the repo config: %v", err)
	}

	if model == "" {
		t.Error("JudgeModel returned an empty model for the repo config")
	}
}
