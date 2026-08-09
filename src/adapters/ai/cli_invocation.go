package ai

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"os/exec"
)

const (
	// cliWaitDelay bounds how long Wait blocks after the process exits
	// or is killed. crush's embedded shell can leave a grandchild
	// holding the stdout pipe; without this the turn would hang forever
	// instead of failing. Mirrors why .claude/scripts/crush-run.sh
	// tree-kills rather than waiting.
	cliWaitDelay = 10 * time.Second
	// transcriptFileMode matches the permissions the generators use for
	// their prompt/response artifacts.
	transcriptFileMode = 0o644
)

// cliInvocation is one subprocess turn: a binary, its argv, the prompt
// piped to stdin, and where the raw transcript is archived.
type cliInvocation struct {
	Binary         string
	Args           []string
	Dir            string
	Env            []string
	Stdin          string
	TranscriptPath string
}

// run spawns the CLI in its own process group and returns its combined
// stdout+stderr. A cancelled context takes down the whole group, not
// just the direct child.
func (inv cliInvocation) run(ctx context.Context) (string, error) {
	//nolint:gosec // binary and argv are engine-owned; no user input reaches them
	cmd := exec.CommandContext(ctx, inv.Binary, inv.Args...)
	cmd.Dir = inv.Dir
	cmd.Env = inv.Env
	cmd.Stdin = strings.NewReader(inv.Stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var output bytes.Buffer

	cmd.Stdout = &output
	cmd.Stderr = &output

	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}

		return nil
	}
	cmd.WaitDelay = cliWaitDelay

	slog.Debug("Spawning agent CLI", "binary", inv.Binary, "args", inv.Args, "dir", inv.Dir)

	runErr := cmd.Run()
	transcript := output.String()

	inv.saveTranscript(transcript)

	if runErr != nil {
		return transcript, fmt.Errorf("%s: %w", inv.Binary, runErr)
	}

	return transcript, nil
}

// saveTranscript archives the raw subprocess output next to the
// prompt/response files the generators already write, so a failed turn
// can be diagnosed without re-running it.
func (inv cliInvocation) saveTranscript(transcript string) {
	if inv.TranscriptPath == "" {
		return
	}

	err := os.WriteFile(inv.TranscriptPath, []byte(transcript), transcriptFileMode)
	if err != nil {
		slog.Warn("Failed to save CLI transcript", "path", inv.TranscriptPath, "error", err)

		return
	}

	slog.Info("CLI transcript saved", "file", inv.TranscriptPath)
}

// composePrompt folds the system prompt into the user prompt for CLIs
// with no native system-prompt flag (crush, codex). The heading makes
// the precedence explicit rather than relying on position alone.
func composePrompt(req Request) string {
	if strings.TrimSpace(req.SystemPrompt) == "" {
		return req.UserPrompt
	}

	var builder strings.Builder

	builder.WriteString("### SYSTEM INSTRUCTIONS (authoritative — follow exactly)\n\n")
	builder.WriteString(req.SystemPrompt)
	builder.WriteString("\n\n### TASK\n\n")
	builder.WriteString(req.UserPrompt)

	return builder.String()
}

// artifactPath builds a per-turn artifact path inside the run's tmp
// dir. An empty TmpDir disables archiving.
func artifactPath(req Request, suffix string) string {
	if req.TmpDir == "" {
		return ""
	}

	label := req.Label
	if label == "" {
		label = "turn"
	}

	return req.TmpDir + "/" + label + "-" + suffix
}
