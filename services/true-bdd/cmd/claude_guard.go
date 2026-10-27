package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func newClaudeGuardCmd() *cobra.Command {
	var policyPath string

	command := &cobra.Command{Use: "claude-guard", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command,
		_ []string) error {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		allowed, reason := decideClaudeHook(policyPath, cmd.InOrStdin())
		if !allowed {
			denyClaudeTool(cmd.ErrOrStderr(), "true-bdd guard: "+reason)
		}

		const decision = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`

		_, err := io.WriteString(cmd.OutOrStdout(), decision+"\n")
		if err != nil {
			return fmt.Errorf("writing claude hook decision: %w", err)
		}

		return nil
	}}
	command.Flags().StringVar(&policyPath, "policy", "", "generated writer policy")

	return command
}

func decideClaudeHook(path string, input io.Reader) (bool, string) {
	raw, err := disk.Read(path)
	if err != nil {
		return false, "cannot read claude guard policy"
	}

	var policy ai.ClaudeGuardPolicy
	if json.Unmarshal(raw, &policy) != nil {
		return false, "invalid claude guard policy"
	}

	payload, err := io.ReadAll(input)
	if err != nil {
		return false, "cannot read claude hook payload"
	}

	return policy.DecidePayload(payload)
}

// denyClaudeTool writes the hook reason and exits with Claude's denial status.
func denyClaudeTool(stderr io.Writer, reason string) {
	_, _ = io.WriteString(stderr, reason+"\n")

	const denyExitCode = 2
	os.Exit(denyExitCode)
}
