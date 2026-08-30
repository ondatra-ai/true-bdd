package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
)

// ProjectDirVar is where Claude Code says it launched, and the working
// directory every hook wants: a gate's paths are written relative to it.
const ProjectDirVar = "CLAUDE_PROJECT_DIR"

// PostToolUseParams is the event, reduced to what a gate acts on. The payload
// carries more — session id, transcript path, the tool's response — and this
// package models a field when something needs it, not before.
type PostToolUseParams struct {
	// ToolName is the tool that just ran, e.g. "Write" or "Edit".
	ToolName string
	// FilePath is what that tool wrote, absolute. Empty for a tool that wrote
	// no file, which is most of them.
	FilePath string
}

// PostToolUseFunc answers one event. A nil error is silence and the tool call
// stands; a non-nil one blocks it, with the error's message as the reason
// Claude Code shows and acts on.
type PostToolUseFunc func(params PostToolUseParams, log *slog.Logger) error

// PostToolUse is the whole of being one: bind the log, enter the project
// directory, read the event off stdin, ask answer, write the verdict.
//
//	tool     names the writer in the Task's shared log
//	taskLog  where that log lives — pkg/ may not import the scripts/ package
//	         that names it, so only the caller can say
func PostToolUse(tool, taskLog string, answer PostToolUseFunc) {
	// Stderr always: stdout carries the verdict, which Claude Code parses.
	logging.Install(logging.Stderr, taskLog, tool)

	err := run(answer)
	if err == nil {
		return
	}

	slog.Error("the hook failed", "error", err)
	// The exit code is the protocol's, so it is owned here rather than by main.
	os.Exit(1)
}

func run(answer PostToolUseFunc) error {
	err := enter()
	if err != nil {
		return err
	}

	payload, err := io.ReadAll(console.In())
	if err != nil {
		return fmt.Errorf("reading the tool payload: %w", err)
	}

	reason, blocked := judge(payload, answer)
	if !blocked {
		return nil
	}

	return block(reason)
}

// enter makes Claude Code's project directory the working directory. git
// answers when the variable is absent, which is how a payload piped in by
// hand still lands somewhere sensible.
func enter() error {
	root := os.Getenv(ProjectDirVar)

	if root == "" {
		top, err := git.TopLevel()
		if err != nil {
			return fmt.Errorf("finding the repository root: %w", err)
		}

		root = top
	}

	resolved, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", root, err)
	}

	err = os.Chdir(resolved)
	if err != nil {
		return fmt.Errorf("moving to %s: %w", resolved, err)
	}

	return nil
}

// judge decodes the event and asks answer about it, reporting whether the tool
// call is blocked and why. An unreadable payload is silence: a hook that
// cannot parse its input has found nothing, which is not finding nothing.
func judge(payload []byte, answer PostToolUseFunc) (string, bool) {
	var event struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
	}

	err := json.Unmarshal(payload, &event)
	if err != nil {
		return "", false
	}

	params := PostToolUseParams{ToolName: event.ToolName, FilePath: event.ToolInput.FilePath}

	found := answer(params, slog.Default().With("hook", "PostToolUse", "tool", params.ToolName))
	if found == nil {
		return "", false
	}

	return found.Error(), true
}
