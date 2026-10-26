package ai

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/codex"
)

const (
	testWorkDir   = "/repo"
	testUserTask  = "validate this"
	testReadSpec  = "Read(**)"
	testGrepSpec  = "Grep(**)"
	testWriteSpec = "Write(./tmp/**)"
)

func editMode() ExecutionMode {
	return ExecutionMode{AllowedTools: []string{testWriteSpec, "Edit(./tmp/**)"}}
}

// sourceEditMode mirrors GetSourceEditMode: tmp writes PLUS a project
// tree the applier is meant to author into.
func sourceEditMode() ExecutionMode {
	mode := editMode()
	mode.AllowedTools = append(mode.AllowedTools, "Write(services/**)")
	mode.SourceWriteRoots = []string{"services/**"}

	return mode
}

func readOnlyMode() ExecutionMode {
	return ExecutionMode{
		AllowedTools:    []string{testReadSpec, testGrepSpec},
		DisallowedTools: []string{bashToolName},
	}
}

// Plan and validation stay read-only independently of the mode's tool grants.
func TestCodexPlansAndValidationStayReadOnly(t *testing.T) {
	t.Parallel()

	if got := codexSandbox(readOnlyMode()); got != codex.SandboxReadOnly {
		t.Errorf("codexSandbox(read-only mode) = %q, want %q", got, codex.SandboxReadOnly)
	}

	if got := codexSandbox(editMode()); got != codex.SandboxReadOnly {
		t.Errorf("codexSandbox(tmp-only writes) = %q, want %q — a scratch write "+
			"grant must not become workspace write access", got, codex.SandboxReadOnly)
	}

	if got := codexSandbox(sourceEditMode()); got != codex.SandboxReadOnly {
		t.Errorf("codexSandbox(source edit mode) = %q, want %q", got, codex.SandboxReadOnly)
	}
}

// Codex has no system-prompt flag, so it has to
// survive inside the user prompt.
func TestComposePromptFoldsInTheSystemPrompt(t *testing.T) {
	t.Parallel()

	composed := composePrompt(Request{SystemPrompt: "BE TERSE", UserPrompt: testUserTask})

	if !strings.Contains(composed, "BE TERSE") || !strings.Contains(composed, testUserTask) {
		t.Fatalf("composed prompt lost content: %q", composed)
	}

	if strings.Index(composed, "BE TERSE") > strings.Index(composed, testUserTask) {
		t.Error("system prompt must precede the task")
	}

	bare := composePrompt(Request{UserPrompt: testUserTask})
	if bare != testUserTask {
		t.Errorf("with no system prompt, composePrompt = %q, want the user prompt verbatim", bare)
	}
}
