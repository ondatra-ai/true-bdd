package ai

import (
	"encoding/json"
	"path/filepath"
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

		return p.Replay.allowsTestCommand(CrushToolCall{Command: payload.ToolInput.Command, WorkingDir: payload.Cwd})
	}

	target := claudeTarget(payload.ToolInput.FilePath, payload.ToolInput.Path, payload.Cwd, p.Replay.ReadRoot)
	switch payload.ToolName {
	case "Read":
		return p.allowsRead(target)
	case "Glob":
		return p.allowsGlob(target, payload.ToolInput.Pattern)
	case "Grep":
		return p.allowsSearch(target)
	case "Write", "Edit", "MultiEdit":
		return p.allowsWrite(target)
	default:
		return false, "tool is not on the claude writer allowlist"
	}
}

func (p ClaudeGuardPolicy) allowsRead(target string) (bool, string) {
	resolved := resolveThroughSymlinks(target)
	// The host always wins, even inside ~/work or a module-cache copy.
	if pathWithinRoot(resolved, resolveThroughSymlinks(p.Replay.ReadRoot)) {
		return true, ""
	}

	for _, root := range p.ExcludedRoots {
		if pathWithinRoot(resolved, resolveThroughSymlinks(root)) {
			return false, "read target is excluded: " + resolved
		}
	}

	for _, root := range p.ReadRoots {
		if pathWithinRoot(resolved, resolveThroughSymlinks(root)) {
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

	if !filepath.IsAbs(target) {
		target = root + string(filepath.Separator) + target
	}

	return target
}

func (p ClaudeGuardPolicy) allowsWrite(target string) (bool, string) {
	resolved := resolveThroughSymlinks(target)
	for _, root := range p.ProtectedRoots {
		if pathWithinRoot(resolved, resolveThroughSymlinks(root)) {
			return false, "the writer cannot change its generated guard configuration"
		}
	}

	return p.Replay.allowsWriteTo(target)
}
