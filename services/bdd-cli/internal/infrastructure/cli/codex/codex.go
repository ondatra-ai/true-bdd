// Package codex is the `codex` command line, one of the two engine wrappers
// that alone may reach pkg/shell from inside the engine. Only the engine spawns
// codex, so the wrapper is engine code and a rebuild regenerates it; the argv
// it builds is stated in docs/architecture/architecture.yaml.
//
// Headless execution facts verified against codex-cli 0.162.1:
//
//	permissions         ordinary turns select -s; the build-code writer
//	                    selects its generated named profile with no approval.
//	the answer is a file stdout carries only a trace; the final message is
//	                    written to the path given to -o.
//	a trailing `-`      is what makes codex read the prompt from stdin.
//	not a git repo      BDD fixture tmpdirs are not, and codex refuses to run
//	                    outside one unless told not to care.
package codex

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "codex"

// Legacy sandboxes retained by every turn except the confined build-code writer.
const (
	SandboxReadOnly       = "read-only"
	SandboxWorkspaceWrite = "workspace-write"
)

// waitDelay bounds how long Wait blocks after the exit, so a grandchild
// holding the stdout pipe cannot hang the turn.
const waitDelay = 10 * time.Second

// Turn is one headless run. Prompt goes to stdin because codex has no
// system-prompt flag, so a system prompt has to survive inside the user's.
type Turn struct {
	// Sandbox is the ordinary turn's -s level; Writer instead uses its generated profile.
	Sandbox string
	// WorkDir is both -C and the child's working directory.
	WorkDir string
	// Model is passed as -m.
	Model string
	// AnswerPath is where codex writes the final message. The caller reads it.
	AnswerPath string
	// Prompt is piped to stdin.
	Prompt string
	// Env is the COMPLETE environment, not a derivation of the parent's.
	Env []string
	// SchemaPath points at a JSON Schema file the answer must satisfy.
	// codex takes a PATH here, unlike claude's inline --json-schema.
	SchemaPath string
	// Writer selects the generated permission profile and JSON events.
	Writer bool
}

// Result retains separate JSON evidence and the started-process fact.
type Result struct {
	Transcript string
	Events     string
	Started    bool
}

// Args is the argv this turn spawns, exported so a caller can assert on it.
func (t Turn) Args() []string {
	args := []string{
		Bin, "exec",
		// No session files: every engine turn is single-shot.
		"--ephemeral",
		"--skip-git-repo-check",
		"--color", "never",
	}
	if t.Writer {
		args = append(args, "--strict-config", "--ignore-rules", "--json")
	} else {
		args = append(args[:2], append([]string{"-s", t.Sandbox}, args[2:]...)...)
	}

	if t.WorkDir != "" {
		args = append(args, "-C", t.WorkDir)
	}

	if t.Model != "" {
		args = append(args, "-m", t.Model)
	}

	if t.SchemaPath != "" {
		args = append(args, "--output-schema", t.SchemaPath)
	}

	return append(args, "-o", t.AnswerPath, "-")
}

// Run runs the turn and returns codex's TRACE — the answer is in AnswerPath.
// The trace comes back even when the turn failed: it is what a caller archives.
func (t Turn) Run() (Result, error) {
	output := shell.Combined()
	if t.Writer {
		output = shell.Capture()
	}

	result, err := shell.Run(context.Background(), t.Args(), shell.Options{
		Dir:       t.WorkDir,
		Env:       shell.Env{}.Exact(t.Env),
		Stdin:     strings.NewReader(t.Prompt),
		Output:    output,
		Group:     true,
		WaitDelay: waitDelay,
	})

	answer := Result{Transcript: result.Stdout + result.Stderr, Started: err == nil}
	if t.Writer {
		answer.Events = result.Stdout
	}

	if err != nil {
		return answer, fmt.Errorf("%s: %w", Bin, err)
	}

	if result.Code != 0 {
		return answer, fmt.Errorf("%s: %w", Bin, result.Err())
	}

	return answer, nil
}
