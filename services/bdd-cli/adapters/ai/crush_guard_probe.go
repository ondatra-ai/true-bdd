package ai

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// crushGuardProbePath is a path no policy can ever grant, so a working
// guard must deny a write to it.
const crushGuardProbePath = "/true-bdd-guard-probe/must-be-denied"

// verifyCrushGuardEnforces proves the guard hook actually denies before any
// crush turn starts: crush FAILS OPEN if the hook can't execute — observed
// directly, a binary with no `crush-guard` subcommand let a write through silently.
func verifyCrushGuardEnforces(ctx context.Context, executable string) error {
	//nolint:gosec // executable is os.Executable(); no user input reaches it
	cmd := exec.CommandContext(ctx, executable, crushGuardSubcommand)
	cmd.Stdin = strings.NewReader("")

	cmd.Env = append(os.Environ(),
		// A policy that grants nothing: every write must be denied.
		CrushPolicyEnvVar+`={"write_roots":[],"allow_bash":false}`,
		CrushToolNameEnvVar+"=write",
		CrushToolFilePathEnvVar+"="+crushGuardProbePath,
	)

	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == CrushGuardDenyExitCode {
		return nil
	}

	return pkgerrors.ErrCrushGuardNotEnforcingAt(executable)
}
