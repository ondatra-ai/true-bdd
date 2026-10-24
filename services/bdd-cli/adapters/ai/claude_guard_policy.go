package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeGuardPolicy confines the writer's tools; Replay reuses #193's exact check.
type ClaudeGuardPolicy struct {
	ProtectedRoots []string         `json:"protected_roots"`
	Replay         CrushGuardPolicy `json:"replay"`
	ReadRoots      []string         `json:"read_roots"`
	ExcludedRoots  []string         `json:"excluded_roots"`
}

// DecidePayload judges Claude Code's PreToolUse payload, failing closed.
func (p ClaudeGuardPolicy) DecidePayload(raw []byte) (bool, string) {
	var payload struct {
		ToolName  string `json:"tool_name"`
		Cwd       string `json:"cwd"`
		ToolInput struct {
			Command    string `json:"command"`
			Background bool   `json:"run_in_background"`
			FilePath   string `json:"file_path"`
			Path       string `json:"path"`
			Pattern    string `json:"pattern"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(raw, &payload) != nil || !filepath.IsAbs(p.Replay.ReadRoot) {
		return false, "invalid claude guard payload or policy"
	}

	if payload.ToolName == "Bash" {
		if payload.ToolInput.Background {
			return false, "background commands are denied; use the foreground"
		}

		return p.allowsClaudeTestCommand(payload.ToolInput.Command, payload.Cwd)
	}

	target := claudeTarget(payload.ToolInput.FilePath, payload.ToolInput.Path, payload.Cwd, p.Replay.ReadRoot)

	return p.allowsFileTool(payload.ToolName, target, payload.ToolInput.Pattern)
}

func (p ClaudeGuardPolicy) allowsFileTool(tool, target, pattern string) (bool, string) {
	if target == "" {
		return false, "cannot resolve the file tool target"
	}

	switch tool {
	case "Read":
		return p.allowsRead(target)
	case "Glob":
		return p.allowsGlob(target, pattern)
	case "Grep":
		return p.allowsSearch(target)
	case "Write", "Edit", "MultiEdit":
		return p.allowsWrite(target)
	default:
		return false, "tool is not on the claude writer allowlist"
	}
}

func (p ClaudeGuardPolicy) allowsClaudeTestCommand(command, cwd string) (bool, string) {
	prefix := "cd " + quoteHookArg(p.Replay.TestDir) + " && "

	resolved := resolveThroughSymlinks(cwd)
	if strings.HasPrefix(command, prefix) &&
		(claudeSamePath(resolved, p.Replay.ReadRoot) || claudeSamePath(resolved, p.Replay.TestDir)) {
		command = strings.TrimPrefix(command, prefix)
		cwd = p.Replay.TestDir
	}

	dir := cwd
	if !filepath.IsAbs(dir) {
		dir = p.Replay.ReadRoot + string(filepath.Separator) + dir
	}

	if filepath.IsAbs(p.Replay.TestDir) && claudeSamePath(dir, p.Replay.TestDir) {
		cwd = p.Replay.TestDir
	}

	return p.Replay.allowsTestCommand(CrushToolCall{Command: command, WorkingDir: cwd})
}

func (p ClaudeGuardPolicy) allowsRead(target string) (bool, string) {
	resolved := resolveThroughSymlinks(target)
	// The host always wins, even inside ~/work or a module-cache copy.
	if claudePathWithinRoot(resolved, resolveThroughSymlinks(p.Replay.ReadRoot)) {
		return true, ""
	}

	for _, root := range p.ExcludedRoots {
		if claudePathWithinRoot(resolved, resolveThroughSymlinks(root)) {
			return false, "read target is excluded: " + resolved
		}
	}

	for _, root := range p.ReadRoots {
		if claudePathWithinRoot(resolved, resolveThroughSymlinks(root)) {
			return true, ""
		}
	}

	return false, "read target is outside the claude writer roots: " + resolved
}

func claudeTarget(filePath, path, cwd, root string) string {
	target := filePath
	if target == "" {
		target = path
	}

	if target == "" {
		target = cwd
	}

	target = expandClaudeHome(target)
	if target == "" {
		return ""
	}

	if !filepath.IsAbs(target) {
		if filepath.IsAbs(cwd) {
			root = cwd
		}

		target = root + string(filepath.Separator) + target
	}

	return target
}

func expandClaudeHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return ""
	}

	return home + strings.TrimPrefix(path, "~")
}

func (p ClaudeGuardPolicy) allowsWrite(target string) (bool, string) {
	resolved := resolveThroughSymlinks(target)
	for _, root := range p.ProtectedRoots {
		if claudePathWithinRoot(resolved, resolveThroughSymlinks(root)) {
			return false, "the writer cannot change its generated guard configuration"
		}
	}

	for _, root := range p.Replay.WriteRoots {
		if claudePathWithinRoot(resolved, root) {
			return true, ""
		}
	}

	return false, "write target is outside the claude writer roots: " + resolved
}
