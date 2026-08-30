package ai

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// crushGuardProbePath is a path no policy can ever grant, so a working
// guard must deny a write to it.
const crushGuardProbePath = "/true-bdd-guard-probe/must-be-denied"

// verifyCrushGuardEnforces proves the guard hook actually denies before any
// crush turn starts: crush FAILS OPEN if the hook can't execute — observed
// directly, a binary with no `crush-guard` subcommand let a write through silently.
func verifyCrushGuardEnforces(executable string) error {
	result, err := spec.Run([]string{executable, crushGuardSubcommand}, cli.Options{
		Stdin: strings.NewReader(""),
		Env: cli.Inherit().Set(
			// A policy that grants nothing: every write must be denied.
			CrushPolicyEnvVar+`={"write_roots":[],"allow_bash":false}`,
			CrushToolNameEnvVar+"=write",
			CrushToolFilePathEnvVar+"="+crushGuardProbePath,
		),
		Output: cli.Discard(),
	})
	if err == nil && result.Code == CrushGuardDenyExitCode {
		return nil
	}

	return pkgerrors.ErrCrushGuardNotEnforcingAt(executable)
}
