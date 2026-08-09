package ai_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
)

// Tool specs and globs reused across the ai_test package.
const (
	readSpec    = "Read(**)"
	grepSpec    = "Grep(**)"
	globSpec    = "Glob(**)"
	writeTmp    = "Write(./tmp/**)"
	editAnySpec = "Edit(**)"
	multiAny    = "MultiEdit(**)"
	bashTool    = "Bash"
	tmpGlob     = "./tmp/**"
	writeTool   = "write"
)

// ExecutionMode is the single permission source for all three CLIs, so
// the projection helpers crush and codex depend on need direct cover.
func TestExecutionModeWriteGlobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode ai.ExecutionMode
		want []string
	}{
		{
			name: "think mode grants only the tmp glob",
			mode: ai.ExecutionMode{
				AllowedTools:    []string{readSpec, writeTmp, globSpec, grepSpec},
				DisallowedTools: []string{bashTool, editAnySpec, multiAny},
			},
			want: []string{tmpGlob},
		},
		{
			name: "edit mode dedupes the repeated glob across write tools",
			mode: ai.ExecutionMode{
				AllowedTools: []string{
					readSpec, writeTmp, "Edit(./tmp/**)", "MultiEdit(./tmp/**)", grepSpec,
				},
				DisallowedTools: []string{bashTool},
			},
			want: []string{tmpGlob},
		},
		{
			name: "read-only tools grant no writes",
			mode: ai.ExecutionMode{AllowedTools: []string{readSpec, grepSpec}},
			want: []string{},
		},
		{
			name: "the zero mode grants no writes",
			mode: ai.ExecutionMode{},
			want: []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := testCase.mode.WriteGlobs()
			if !slices.Equal(got, testCase.want) {
				t.Errorf("WriteGlobs() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestExecutionModeAllowsBash(t *testing.T) {
	t.Parallel()

	denied := ai.ExecutionMode{DisallowedTools: []string{bashTool, "Agent", "Task"}}
	if denied.AllowsBash() {
		t.Error("AllowsBash() = true although Bash is disallowed")
	}

	if !(ai.ExecutionMode{}).AllowsBash() {
		t.Error("AllowsBash() = false for a mode with no disallow list")
	}
}
