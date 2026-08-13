package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

// newCrushGuardCmd builds the hidden PreToolUse hook that the crush
// provider wires into its generated config.
//
// `crush run` has no permission gate of its own, so this is the ONLY
// enforcement on what a crush-backed turn may write. Shipping it as a
// subcommand of this binary means a host project needs no guard script
// of its own — the crush provider points the hook at os.Executable().
//
// It fails CLOSED: a missing or malformed policy denies everything, so
// a stray `crush run` that happens to inherit the config stays
// read-only.
func newCrushGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "crush-guard",
		Short:  "PreToolUse write-guard for engine-driven crush turns",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Cobra would otherwise print usage on the non-zero exit we
			// use to signal a denial.
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			runCrushGuard(cmd.InOrStdin(), cmd.ErrOrStderr())

			return nil
		},
	}
}

// runCrushGuard decides one tool call and exits with crush's verdict
// code.
func runCrushGuard(stdin io.Reader, stderr io.Writer) {
	policy, err := ai.LoadCrushGuardPolicy()
	if err != nil {
		denyCrushTool(stderr, "true-bdd guard: "+err.Error())
	}

	toolName, targetPath := crushToolCall(stdin)

	allowed, reason := policy.Decide(toolName, targetPath)
	if !allowed {
		denyCrushTool(stderr, "true-bdd guard: "+reason)
	}
}

// denyCrushTool writes the reason and exits with the deny code.
func denyCrushTool(stderr io.Writer, reason string) {
	_, _ = io.WriteString(stderr, reason+"\n")

	os.Exit(ai.CrushGuardDenyExitCode)
}

// crushHookPayload is the subset of the JSON crush pipes to a hook on
// stdin. Environment variables are the primary channel; this is the
// fallback for the fields they omit.
type crushHookPayload struct {
	ToolName string `json:"tool_name"`
	Input    struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	} `json:"input"`
}

// crushToolCall reads the tool name and target path, preferring the
// environment variables and falling back to the stdin payload.
func crushToolCall(stdin io.Reader) (string, string) {
	toolName := strings.TrimSpace(os.Getenv(ai.CrushToolNameEnvVar))
	targetPath := strings.TrimSpace(os.Getenv(ai.CrushToolFilePathEnvVar))

	if toolName != "" && targetPath != "" {
		return toolName, targetPath
	}

	payload := decodeCrushHookPayload(stdin)

	if toolName == "" {
		toolName = payload.ToolName
	}

	if targetPath == "" {
		targetPath = payload.Input.FilePath
	}

	if targetPath == "" {
		targetPath = payload.Input.Path
	}

	return toolName, targetPath
}

// decodeCrushHookPayload best-effort parses the stdin payload. A
// missing or unreadable payload yields a zero value, which — with no
// usable tool name — denies by default.
func decodeCrushHookPayload(stdin io.Reader) crushHookPayload {
	var payload crushHookPayload

	if stdin == nil {
		return payload
	}

	raw, err := io.ReadAll(stdin)
	if err != nil || len(raw) == 0 {
		return payload
	}

	_ = json.Unmarshal(raw, &payload)

	return payload
}
