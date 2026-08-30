// Package codex is the `codex` command line, one of the typed wrappers
// pkg/shell may be reached through.
//
// Four facts about the binary shape this package, none documented upstream:
//
//	-s is mandatory     without a sandbox level `codex exec` blocks on an
//	                    approval prompt and hangs forever headlessly.
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

// The two usable sandbox levels. There is nothing between them: codex either
// writes nothing or writes the whole working root.
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
	// Sandbox is the -s level; empty is not valid — see the package doc.
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
}

// Args is the argv this turn spawns, exported so a caller can assert on it.
func (t Turn) Args() []string {
	args := []string{
		Bin, "exec",
		"-s", t.Sandbox,
		// No session files: every engine turn is single-shot.
		"--ephemeral",
		"--skip-git-repo-check",
		"--color", "never",
	}

	if t.WorkDir != "" {
		args = append(args, "-C", t.WorkDir)
	}

	if t.Model != "" {
		args = append(args, "-m", t.Model)
	}

	return append(args, "-o", t.AnswerPath, "-")
}

// Run runs the turn and returns codex's TRACE — the answer is in AnswerPath.
// The trace comes back even when the turn failed: it is what a caller archives.
func (t Turn) Run() (string, error) {
	result, err := shell.Run(context.Background(), t.Args(), shell.Options{
		Dir:       t.WorkDir,
		Env:       shell.Env{}.Exact(t.Env),
		Stdin:     strings.NewReader(t.Prompt),
		Output:    shell.Combined(),
		Group:     true,
		WaitDelay: waitDelay,
	})
	if err != nil {
		return result.Stdout, fmt.Errorf("%s: %w", Bin, err)
	}

	if result.Code != 0 {
		return result.Stdout, fmt.Errorf("%s: %w", Bin, result.Err())
	}

	return result.Stdout, nil
}
