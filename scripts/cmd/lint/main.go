// Command lint runs this repository's source-quality gates.
//
// It replaces scripts/lints.sh and the four lint-*.sh scripts it drove, and
// .claude/hooks/lint-changed.sh. Every gate answers for the files it is given
// and for those alone; with none, for the whole repository.
//
//	lint                      every gate, report only (what CI runs)
//	lint <file>...            the gates those files select, with --fix
//	lint comments [file...]   the comment budget
//	lint schemas [file...]    yamale, document against schema
//	lint markdown [file...]   markdownlint-cli2
//	lint claude-md            CLAUDE.md width and the upstream mirror
//	lint hook                 PostToolUse; the tool payload arrives on stdin
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/lint"
)

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}

	// The gates report their own findings; anything else is a diagnosis.
	if !errors.Is(err, lint.ErrFailed) {
		fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(1)
}

// run answers from the repository root, which is where every gate's paths,
// pathspecs and tool configs are written relative to.
func run(args []string) error {
	err := os.Chdir(repoRoot())
	if err != nil {
		return fmt.Errorf("moving to the repository root: %w", err)
	}

	if len(args) == 0 {
		return lint.Dispatch(os.Stdout, nil) //nolint:wrapcheck // the gate's own verdict.
	}

	//nolint:wrapcheck // every branch returns a gate's own verdict, already reported.
	switch args[0] {
	case "comments":
		return lint.Comments(os.Stdout, args[1:])
	case "schemas":
		return lint.Schemas(os.Stdout, args[1:])
	case "markdown":
		return lint.Markdown(os.Stdout, args[1:])
	case "claude-md":
		return lint.ClaudeMD(os.Stdout)
	case "hook":
		return lint.Hook(os.Stdin, os.Stdout)
	default:
		return lint.Dispatch(os.Stdout, args)
	}
}

// repoRoot prefers the directory Claude Code names when it invokes a hook,
// and falls back to git for every other caller.
func repoRoot() string {
	if dir := os.Getenv("CLAUDE_PROJECT_DIR"); dir != "" {
		resolved, err := filepath.Abs(dir)
		if err == nil {
			return resolved
		}
	}

	out, err := exec.CommandContext(context.Background(),
		"git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "."
	}

	return strings.TrimSpace(string(out))
}
