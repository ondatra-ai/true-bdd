package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

// newCrushGuardCmd builds the hidden PreToolUse hook the crush provider
// wires into its generated config (see CrushGuardPolicy), shipped as a
// subcommand of this binary so a host project needs no guard script of its own.
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

	call := crushToolCall(stdin)

	allowed, reason := policy.DecideCall(call)
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
	ToolName  string         `json:"tool_name"`
	Input     crushHookInput `json:"input"`
	ToolInput crushHookInput `json:"tool_input"`
}

type crushHookInput struct {
	FilePath   string `json:"file_path"`
	Path       string `json:"path"`
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir"`
}

// crushToolCall reads environment fields first, then the hook payload.
func crushToolCall(stdin io.Reader) ai.CrushToolCall {
	payload := decodeCrushHookPayload(stdin)

	input := payload.ToolInput
	if input == (crushHookInput{}) {
		input = payload.Input
	}

	call := ai.CrushToolCall{
		Name:       strings.TrimSpace(os.Getenv(ai.CrushToolNameEnvVar)),
		TargetPath: os.Getenv(ai.CrushToolFilePathEnvVar),
		Command:    os.Getenv("CRUSH_TOOL_INPUT_COMMAND"),
		WorkingDir: input.WorkingDir,
	}
	if call.Name == "" {
		call.Name = payload.ToolName
	}

	if strings.TrimSpace(call.TargetPath) == "" {
		call.TargetPath = input.FilePath
	}

	if strings.TrimSpace(call.TargetPath) == "" {
		call.TargetPath = input.Path
	}

	if call.Command == "" {
		call.Command = input.Command
	}

	return call
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
