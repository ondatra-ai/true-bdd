package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

const (
	// crushGlobalConfigEnvVar points crush at a config DIRECTORY that
	// the engine generates per run. Verified against crush v0.88.1: the
	// directory's crush.json joins the config load chain, which lets the
	// engine supply permissions and hooks WITHOUT writing into the host
	// repo or clobbering an existing .crush.json.
	crushGlobalConfigEnvVar = "CRUSH_GLOBAL_CONFIG"
	// crushGuardSubcommand is the hidden true-bdd subcommand crush
	// invokes as its PreToolUse hook.
	crushGuardSubcommand = "crush-guard"
	// crushGuardTimeoutSeconds bounds one guard invocation.
	crushGuardTimeoutSeconds = 15
	// crushConfigDirMode / crushConfigFileMode for the generated config.
	crushConfigDirMode  = 0o755
	crushConfigFileMode = 0o644
)

// CrushProvider runs a turn through the `crush` CLI.
//
// Two properties of crush shape this implementation, both verified
// against the live binary:
//
//   - `crush run` has NO permission gate, so writes are policed by a
//     generated PreToolUse hook (see CrushGuardPolicy) rather than by
//     the CLI itself.
//   - crush SILENTLY ignores an unknown model pinned in config — a bad
//     pin falls back to whatever global state says instead of failing.
//     The model is therefore always passed as `-m`, never via the
//     generated file.
type CrushProvider struct{}

// NewCrushProvider creates the crush provider.
func NewCrushProvider() *CrushProvider {
	return &CrushProvider{}
}

// Name returns the CLI's config name.
func (p *CrushProvider) Name() string {
	return "crush"
}

// Execute runs one turn, guarded by a generated per-run config.
func (p *CrushProvider) Execute(ctx context.Context, req Request) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), err)
	}

	// crush fails OPEN on a hook it cannot run, and the hook is the
	// only write gate a crush turn has. Prove it denies before letting
	// the turn start.
	err = verifyCrushGuardEnforces(ctx, executable)
	if err != nil {
		return "", err
	}

	configDir, err := writeCrushConfig(req, executable)
	if err != nil {
		return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), err)
	}

	env, err := buildCrushEnv(req, configDir)
	if err != nil {
		return "", pkgerrors.ErrProviderExecutionFailed(p.Name(), err)
	}

	warnOnHostCrushConfig(req.WorkDir)

	invocation := cliInvocation{
		Binary:         "crush",
		Args:           buildCrushArgs(req),
		Dir:            req.WorkDir,
		Env:            env,
		Stdin:          composePrompt(req),
		TranscriptPath: artifactPath(req, "crush.log"),
	}

	transcript, runErr := invocation.run(ctx)
	if runErr != nil {
		return transcript, pkgerrors.ErrProviderExecutionFailed(p.Name(), runErr)
	}

	if strings.TrimSpace(transcript) == "" {
		return "", pkgerrors.ErrProviderProducedNoOutput(p.Name())
	}

	return transcript, nil
}

// buildCrushArgs builds the argv for one non-interactive turn. Kept
// pure so it is unit-testable without spawning crush.
func buildCrushArgs(req Request) []string {
	args := []string{"run", "--quiet"}

	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}

	if req.WorkDir != "" {
		args = append(args, "--cwd", req.WorkDir)
	}

	return args
}

// crushConfig is the subset of crush's config schema the engine
// generates. The model is deliberately absent — see CrushProvider.
type crushConfig struct {
	Permissions crushPermissions `json:"permissions"`
	Hooks       crushHooks       `json:"hooks"`
}

type crushPermissions struct {
	AllowedTools []string `json:"allowed_tools"`
}

type crushHooks struct {
	// PreToolUse is crush's own event name; the casing is its contract,
	// not ours.
	//nolint:tagliatelle // external tool's schema key
	PreToolUse []crushHook `json:"PreToolUse"`
}

type crushHook struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// writeCrushConfig generates the per-run config directory and returns
// its path for CRUSH_GLOBAL_CONFIG.
func writeCrushConfig(req Request, executable string) (string, error) {
	err := os.MkdirAll(req.TmpDir, crushConfigDirMode)
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(req.TmpDir, err)
	}

	// One directory per turn. A fixed name under a shared TmpDir means
	// two concurrent turns write the same crush.json, and the second
	// would hand the first turn's policy — including its write roots —
	// to a turn that was granted something narrower.
	configDir, err := os.MkdirTemp(req.TmpDir, "crush-config-")
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(req.TmpDir, err)
	}

	guardCommand := crushGuardCommand(executable)

	config := crushConfig{
		Permissions: crushPermissions{AllowedTools: crushAllowedTools(req.Mode)},
		Hooks: crushHooks{PreToolUse: []crushHook{{
			Name:    "true-bdd-guard",
			Command: guardCommand,
			Timeout: crushGuardTimeoutSeconds,
		}}},
	}

	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(configDir, err)
	}

	configPath := filepath.Join(configDir, "crush.json")

	err = os.WriteFile(configPath, encoded, crushConfigFileMode)
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(configPath, err)
	}

	return configDir, nil
}

// crushAllowedTools lists the tools crush may use for this mode. The
// hook is the real gate; this list keeps crush from balking before the
// hook ever runs.
func crushAllowedTools(mode ExecutionMode) []string {
	allowed := append([]string{}, crushReadOnlyTools()...)

	if len(mode.WriteGlobs()) > 0 {
		allowed = append(allowed, crushFileTools()...)
	}

	if mode.AllowsBash() {
		allowed = append(allowed, "bash")
	}

	return allowed
}

// crushGuardCommand builds the hook command line pointing back at this
// binary, so a host project needs no guard script of its own.
//
// The path is always quoted, never conditionally. crush parses this
// string as a shell command, and a binary living under a path
// containing a quote, `$`, or `;` would otherwise be emitted raw — at
// best the hook fails to start, and crush FAILS OPEN when a hook cannot
// run, so a quoting slip silently removes the only write gate.
func crushGuardCommand(executable string) string {
	return shellQuote(executable) + " " + crushGuardSubcommand
}

// shellQuote wraps a value in single quotes, which suppress every shell
// metacharacter, escaping any embedded single quote by closing the
// quoted run, emitting an escaped quote, and reopening it.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// buildCrushEnv layers the generated config dir and the guard policy
// onto the inherited environment. Later entries win in os/exec, so
// these override anything already set.
func buildCrushEnv(req Request, configDir string) ([]string, error) {
	policyEntry, err := NewCrushGuardPolicy(req.Mode, req.WorkDir).Encode()
	if err != nil {
		return nil, err
	}

	return append(os.Environ(),
		crushGlobalConfigEnvVar+"="+configDir,
		policyEntry,
	), nil
}

// warnOnHostCrushConfig notes that a host repo's own crush config
// merges over the engine's generated one and may widen `permissions`.
// Verified against crush v0.88.1: PreToolUse hooks are ADDITIVE — a
// host hook does not displace the engine's, and a denial from either
// blocks the tool — so writes stay policed. The operator should still
// know another config is in play.
func warnOnHostCrushConfig(workDir string) {
	if workDir == "" {
		return
	}

	for _, name := range []string{".crush.json", "crush.json"} {
		_, err := os.Stat(filepath.Join(workDir, name))
		if err == nil {
			slog.Warn("Host crush config merges over the engine's generated config",
				"file", filepath.Join(workDir, name),
				"note", "engine write-guard hook still applies")
		}
	}
}
