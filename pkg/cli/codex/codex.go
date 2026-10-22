// Package codex is the scripts' typed interface to codex-cli.
package codex

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Options describes one delegated turn, preserving policy on resume.
type Options struct {
	RunMarker string
	Dir       string
	Final     string
	Writer    bool
	Search    bool
	Output    io.Writer
	Watchdog  shell.Watchdog
}

// Result is the CLI's exit code and signal.
type Result = shell.Result

// LoginStatus is empty when authenticated, otherwise signed-out or unavailable.
func LoginStatus(ctx context.Context, dir string) string {
	const timeout = 30 * time.Second

	return CheckLogin(ctx, dir, shell.Watchdog{Stall: timeout, Limit: timeout, Grace: time.Second})
}

// CheckLogin checks authentication under the caller's preparation watchdog.
func CheckLogin(ctx context.Context, dir string, watch shell.Watchdog) string {
	result, err := shell.Watch(ctx, []string{"codex", "login", "status"}, shell.Options{Dir: dir}, watch)

	text := result.Stdout + result.Stderr
	if err == nil && result.Code == 0 && strings.Contains(text, "Logged in using ") {
		return ""
	}

	if strings.Contains(text, "Not logged in") {
		return "signed-out"
	}

	return "unavailable"
}

// StartRun starts a fresh session with the brief on stdin.
func StartRun(ctx context.Context, brief string, opt Options) (Result, error) {
	return execute(ctx, "", brief, opt)
}

// ResumeRun continues exactly the named session with a retry note on stdin.
func ResumeRun(ctx context.Context, session, note string, opt Options) (Result, error) {
	return execute(ctx, session, note, opt)
}

func execute(ctx context.Context, session, prompt string, opt Options) (Result, error) {
	output := shell.Capture()
	if opt.Output != nil {
		output = shell.To(opt.Output)
	}

	return shell.Watch(ctx, append([]string{"codex"}, opt.args(session)...),
		shell.Options{Dir: opt.Dir, Stdin: strings.NewReader(prompt), Output: output,
			Env: shell.Inherit().Set("TRUE_BDD_CODEX_RUN=" + opt.RunMarker)}, opt.Watchdog)
}

func (o Options) args(session string) []string {
	var args []string
	if o.Search {
		args = append(args, "--search")
	}

	sandbox := "read-only"
	if o.Writer {
		sandbox = "danger-full-access"
	}

	args = append(args, "exec", "-s", sandbox, "-c", `approval_policy="never"`)
	if o.Writer {
		args = append(args, "-c", `model_reasoning_effort="high"`)
	}

	if !o.Search {
		args = append(args, "-c", `web_search="disabled"`)
	}

	args = append(args, "-C", o.Dir)
	if session != "" {
		args = append(args, "resume", session)
	}

	return append(args, "-o", o.Final, "-")
}

// Watchdog is shared with preparation commands and the writer fallback.
type Watchdog = shell.Watchdog
