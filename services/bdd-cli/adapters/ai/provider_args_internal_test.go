package ai

import (
	"slices"
	"strings"
	"testing"
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

// The model MUST arrive as `-m`: crush silently ignores an unknown
// model pinned in config and falls back to global state, so a config
// pin would fail invisibly.
func TestBuildCrushArgs(t *testing.T) {
	t.Parallel()

	args := buildCrushArgs(Request{Model: "zhipu-coding/glm-5.2", WorkDir: testWorkDir})

	want := []string{"run", "--quiet", "-m", "zhipu-coding/glm-5.2", "--cwd", testWorkDir}
	if !slices.Equal(args, want) {
		t.Fatalf("buildCrushArgs = %v, want %v", args, want)
	}
}

func TestCrushAllowedToolsTracksTheMode(t *testing.T) {
	t.Parallel()

	writable := crushAllowedTools(editMode())
	if !slices.Contains(writable, "write") || !slices.Contains(writable, "bash") {
		t.Errorf("edit mode allowed tools = %v, want write and bash present", writable)
	}

	readOnly := crushAllowedTools(readOnlyMode())
	if slices.Contains(readOnly, "write") || slices.Contains(readOnly, "bash") {
		t.Errorf("read-only allowed tools = %v, want neither write nor bash", readOnly)
	}
}

func TestBuildCodexArgs(t *testing.T) {
	t.Parallel()

	args := buildCodexArgs(Request{
		Model:   "gpt-5.6-sol",
		WorkDir: testWorkDir,
		Mode:    sourceEditMode(),
	}, "/tmp/run/answer.md")

	// `-s` is mandatory: without it `codex exec` blocks on an approval
	// prompt and hangs headlessly forever.
	if !slices.Contains(args, "-s") || !slices.Contains(args, codexSandboxWorkspaceWrite) {
		t.Errorf("args = %v, want a workspace-write sandbox flag", args)
	}

	// Fixture tmpdirs are not git repos.
	if !slices.Contains(args, "--skip-git-repo-check") {
		t.Errorf("args = %v, want --skip-git-repo-check", args)
	}

	if !slices.Contains(args, "--ephemeral") {
		t.Errorf("args = %v, want --ephemeral", args)
	}

	// The answer is read from the -o file; stdout is only a trace. And
	// the trailing `-` is what makes codex read the prompt from stdin.
	if args[len(args)-3] != "-o" || args[len(args)-2] != "/tmp/run/answer.md" || args[len(args)-1] != "-" {
		t.Errorf("args tail = %v, want [-o <path> -]", args[len(args)-3:])
	}
}

// codex has no sandbox level between "nothing" and "all of the working
// root", so only a mode that deliberately opens a project tree may have
// workspace-write. A tmp-only write grant must NOT escalate: that would
// hand a validation or fix turn write access to the tree it is only
// supposed to read.
func TestCodexSandboxOnlyEscalatesForSourceWrites(t *testing.T) {
	t.Parallel()

	if got := codexSandbox(readOnlyMode()); got != codexSandboxReadOnly {
		t.Errorf("codexSandbox(read-only mode) = %q, want %q", got, codexSandboxReadOnly)
	}

	if got := codexSandbox(editMode()); got != codexSandboxReadOnly {
		t.Errorf("codexSandbox(tmp-only writes) = %q, want %q — a scratch write "+
			"grant must not become workspace write access", got, codexSandboxReadOnly)
	}

	if got := codexSandbox(sourceEditMode()); got != codexSandboxWorkspaceWrite {
		t.Errorf("codexSandbox(source edit mode) = %q, want %q", got, codexSandboxWorkspaceWrite)
	}
}

// Neither crush nor codex has a system-prompt flag, so it has to
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
