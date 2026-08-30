package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/crush"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const (
	// crushGlobalConfigEnvVar points crush at a generated config DIRECTORY.
	// Verified against crush v0.88.1: its crush.json joins the config load
	// chain, so the engine supplies permissions/hooks without touching the host's .crush.json.
	crushGlobalConfigEnvVar = crush.GlobalConfigVar
	// crushGuardSubcommand is the hidden true-bdd subcommand crush
	// invokes as its PreToolUse hook.
	crushGuardSubcommand = "crush-guard"
	// crushGuardTimeoutSeconds bounds one guard invocation.
	crushGuardTimeoutSeconds = 15
	// crushConfigDirMode / crushConfigFileMode for the generated config.
)

// CrushProvider runs a turn through the `crush` CLI. Verified against the
// live binary: it has NO permission gate (see CrushGuardPolicy), and it
// SILENTLY ignores an unknown model pin, so the model is always passed as `-m`.
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
	err = verifyCrushGuardEnforces(executable)
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

	turn := crush.Turn{
		Model:   req.Model,
		WorkDir: req.WorkDir,
		Prompt:  composePrompt(req),
		Env:     env,
	}

	transcript, runErr := turn.Run()

	saveTranscript(artifactPath(req, "crush.log"), transcript)

	if runErr != nil {
		return transcript, pkgerrors.ErrProviderExecutionFailed(p.Name(), runErr)
	}

	if strings.TrimSpace(transcript) == "" {
		return "", pkgerrors.ErrProviderProducedNoOutput(p.Name())
	}

	return transcript, nil
}

// crushConfig is the subset of crush's config schema the engine
// generates. The model is deliberately absent — see CrushProvider.
type crushConfig struct {
	Options     crushOptions     `json:"options"`
	Permissions crushPermissions `json:"permissions"`
	Hooks       crushHooks       `json:"hooks"`
}

type crushOptions struct {
	// DataDirectory is crush's SQLite state dir. It defaults to `.crush`
	// under the working directory (i.e., inside the host repo); pinning
	// it under the run's TmpDir keeps crush from writing there.
	DataDirectory string `json:"data_directory"`
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
	err := disk.Dir(req.TmpDir, disk.Shared)
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(req.TmpDir, err)
	}

	// One directory per turn: a fixed name under a shared TmpDir would let
	// two concurrent turns share crush.json, handing the second turn the
	// first's write roots — possibly broader than what it was actually granted.
	configDir, err := disk.TempDir(req.TmpDir, "crush-config-")
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(req.TmpDir, err)
	}

	guardCommand := crushGuardCommand(executable)

	config := crushConfig{
		Options:     crushOptions{DataDirectory: crushDataDir(req, configDir)},
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

	err = disk.Write(configPath, encoded, disk.Shared)
	if err != nil {
		return "", pkgerrors.ErrWriteProviderConfigFailed(configPath, err)
	}

	return configDir, nil
}

// crushDataDir resolves the per-turn data dir. TmpDir is usually relative,
// and crush resolves a relative data_directory against its own cwd (the
// host project root) — same anchor today, but absolutizing removes that coupling.
func crushDataDir(req Request, configDir string) string {
	dataDir := filepath.Join(configDir, "crush-data")
	if filepath.IsAbs(dataDir) {
		return dataDir
	}

	return filepath.Join(req.WorkDir, dataDir)
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

// crushGuardCommand builds the hook command pointing back at this binary.
// The path is always quoted, never conditionally: crush parses it as a
// shell command, and it FAILS OPEN, so an unquoted quote/`$`/`;` would silently kill the only write gate.
func crushGuardCommand(executable string) string {
	return crush.Quote(executable) + " " + crushGuardSubcommand
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

// warnOnHostCrushConfig warns when a host config merges over the engine's:
// verified v0.88.1, PreToolUse hooks are ADDITIVE, so writes stay policed
// either way. It walks UP from workdir because a parent-only config once silently blocked every write.
func warnOnHostCrushConfig(workDir string) {
	if workDir == "" {
		return
	}

	dir, err := filepath.Abs(workDir)
	if err != nil {
		return
	}

	for {
		for _, name := range []string{".crush.json", "crush.json"} {
			path := filepath.Join(dir, name)

			keys, found := hostCrushConfigKeys(path)
			if found {
				slog.Warn("Host crush config merges over the engine's generated config",
					"file", path,
					"declares", keys,
					"note", "engine write-guard hook still applies")
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}

		dir = parent
	}
}

// hostCrushConfigKeys reports the enforcement-relevant top-level keys a
// host config declares — only `permissions` and `hooks` can narrow what
// the apply turn may do, so those are the only ones worth a warning.
func hostCrushConfigKeys(path string) ([]string, bool) {
	raw, err := disk.Read(path)
	if err != nil {
		return nil, false
	}

	var decoded map[string]json.RawMessage

	err = json.Unmarshal(raw, &decoded)
	if err != nil {
		// Unparseable, but crush will still try to merge it — say so.
		return []string{"unparseable"}, true
	}

	var keys []string

	for _, name := range []string{"permissions", "hooks"} {
		_, ok := decoded[name]
		if ok {
			keys = append(keys, name)
		}
	}

	return keys, len(keys) > 0
}
