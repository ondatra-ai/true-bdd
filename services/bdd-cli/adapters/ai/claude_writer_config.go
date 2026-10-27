package ai

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const claudeHooksKey = "hooks"
const claudeGuardTimeoutSeconds = 15

func claudeWriterOptions(ctx context.Context, req Request) (*claude.WriterOptions, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locating claude guard: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locating claude state: %w", err)
	}

	session, err := newClaudeSessionID()
	if err != nil {
		return nil, err
	}

	configRoot := os.Getenv("CLAUDE_CONFIG_DIR")
	if configRoot == "" {
		configRoot = filepath.Join(home, ".claude")
	}

	project := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(resolveThroughSymlinks(req.WorkDir), "-")
	spills := filepath.Join(configRoot, "projects", project, session, "tool-results")

	err = disk.Dir(spills, disk.Private)
	if err != nil {
		return nil, fmt.Errorf("creating claude spill directory: %w", err)
	}

	roots, err := gotool.ReadRoots(ctx, req.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("claude writer toolchain: %w", err)
	}

	const requiredRoots = 2
	if len(roots) != requiredRoots {
		return nil, fmt.Errorf("claude writer toolchain: %w", pkgerrors.ErrClaudeStreamNoOutput)
	}

	roots = append(roots, spills)
	dir := artifactPath(req, "claude-config")
	readRoots := append([]string{roots[0]}, req.Mode.LibraryReadRoots...)
	readRoots = append(readRoots, spills)
	policy := ClaudeGuardPolicy{LibraryCache: roots[1],
		ProtectedRoots: []string{dir, filepath.Join(req.WorkDir, "tmp/library-download")},
		Replay:         NewReplayPolicy(req.Mode, req.WorkDir),
		ReadRoots:      readRoots, ExcludedRoots: []string{}}

	policyPath := filepath.Join(dir, "policy.json")

	err = writeClaudeJSON(policyPath, policy)
	if err != nil {
		return nil, err
	}

	settingsPath := filepath.Join(dir, "settings.json")
	command := quoteHookArg(executable) + " claude-guard --policy " + quoteHookArg(policyPath)

	err = writeClaudeJSON(settingsPath, claudeWriterSettings(command))
	if err != nil {
		return nil, err
	}

	return &claude.WriterOptions{Settings: settingsPath, ReadDirs: roots, SessionID: session}, nil
}

func writeClaudeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding claude configuration: %w", err)
	}

	err = disk.Write(path, append(raw, '\n'), disk.Private)
	if err != nil {
		return fmt.Errorf("writing claude configuration: %w", err)
	}

	return nil
}

func newClaudeSessionID() (string, error) {
	var data [16]byte

	_, err := rand.Read(data[:])
	if err != nil {
		return "", fmt.Errorf("creating claude session id: %w", err)
	}

	const (
		versionMask = 0x0f
		variantMask = 0x3f
		versionFour = 0x40
		variantRFC  = 0x80
	)

	data[6] = (data[6] & versionMask) | versionFour
	data[8] = (data[8] & variantMask) | variantRFC

	return fmt.Sprintf("%x-%x-%x-%x-%x", data[:4], data[4:6], data[6:8], data[8:10], data[10:]), nil
}

func quoteHookArg(value string) string { return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'" }

func claudeWriterSettings(command string) map[string]any {
	return map[string]any{
		"permissions": map[string]any{"defaultMode": "default", "allow": []string{"Read", "Glob", "Grep"},
			"deny": []string{"Agent", "Task", "WebFetch", "WebSearch", "NotebookEdit"}},
		claudeHooksKey: map[string]any{"PreToolUse": []any{map[string]any{"matcher": "*",
			claudeHooksKey: []any{map[string]any{"type": "command", "command": command,
				"timeout": claudeGuardTimeoutSeconds}}}}},
		"claudeMdExcludes": []string{"**"}, "autoMemoryEnabled": false, "enabledPlugins": map[string]bool{},
		"env": map[string]string{"BASH_DEFAULT_TIMEOUT_MS": "600000", "BASH_MAX_TIMEOUT_MS": "600000"},
	}
}
