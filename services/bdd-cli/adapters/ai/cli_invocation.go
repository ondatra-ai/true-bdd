package ai

import (
	"context"
	"fmt"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const (
	// cliWaitDelay bounds how long Wait blocks after exit: crush's embedded
	// shell can leave a grandchild holding the stdout pipe, so without this
	// the turn hangs forever — also why Cancel below kills the whole process group.
	cliWaitDelay = 10 * time.Second
	// transcriptFileMode matches the permissions the generators use for
	// their prompt/response artifacts.
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
	slog.Debug("Spawning agent CLI", "binary", inv.Binary, "args", inv.Args, "dir", inv.Dir)

	result, runErr := spec.Run(ctx, append([]string{inv.Binary}, inv.Args...), cli.Options{
		Dir:       inv.Dir,
		Env:       cli.Exact(inv.Env),
		Stdin:     strings.NewReader(inv.Stdin),
		Output:    cli.Combined(),
		Group:     true,
		WaitDelay: cliWaitDelay,
	})

	transcript := result.Stdout

	inv.saveTranscript(transcript)

	if runErr != nil {
		return transcript, fmt.Errorf("%s: %w", inv.Binary, runErr)
	}

	if result.Code != 0 {
		return transcript, fmt.Errorf("%s: %w", inv.Binary, result.Err())
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

	err := disk.Write(inv.TranscriptPath, []byte(transcript), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save CLI transcript", "path", inv.TranscriptPath, "error", err)

		return
	}

	slog.Info(enginelog.MsgTranscriptSaved, "file", inv.TranscriptPath)
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
