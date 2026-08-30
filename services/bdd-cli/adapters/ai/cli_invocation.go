package ai

import (
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
)

// saveTranscript archives the raw subprocess output next to the
// prompt/response files the generators already write, so a failed turn
// can be diagnosed without re-running it.
func saveTranscript(path, transcript string) {
	if path == "" {
		return
	}

	err := disk.Write(path, []byte(transcript), disk.Shared)
	if err != nil {
		slog.Warn("Failed to save CLI transcript", "path", path, "error", err)

		return
	}

	slog.Info(enginelog.MsgTranscriptSaved, "file", path)
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
