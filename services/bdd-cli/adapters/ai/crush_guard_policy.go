package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const (
	// CrushPolicyEnvVar carries the JSON-encoded CrushGuardPolicy from
	// the crush provider to the `true-bdd crush-guard` hook process
	// that crush spawns before every tool call.
	CrushPolicyEnvVar = "TRUE_BDD_CRUSH_POLICY"
	// CrushToolNameEnvVar / CrushToolFilePathEnvVar are the tool
	// details crush exports to a PreToolUse hook.
	CrushToolNameEnvVar     = "CRUSH_TOOL_NAME"
	CrushToolFilePathEnvVar = "CRUSH_TOOL_INPUT_FILE_PATH"
	// CrushGuardDenyExitCode is crush's hook contract: 0 allows the
	// tool call, 2 denies it with stderr carrying the reason.
	CrushGuardDenyExitCode = 2
)

// crushReadOnlyTools are tools that cannot mutate anything, so they're
// always safe. Everything else is DENIED by default (fail closed): a
// deny-list would let an unlisted writer like `download` slip through.
func crushReadOnlyTools() []string {
	return []string{"view", "ls", "grep", "glob", "sourcegraph", "diagnostics", "fetch"}
}

// crushFileTools are the tools that create or modify files.
func crushFileTools() []string {
	return []string{"edit", "write", "multiedit", "patch", "create", "download"}
}

// crushReadOnlyToolPrefixes cover MCP tool namespaces that only read.
func crushReadOnlyToolPrefixes() []string {
	return []string{"mcp_context7", "context7"}
}

// CrushGuardPolicy is the write gate for one crush turn, enforced by a
// PreToolUse hook — crush run has no permission gate of its own — and
// derived from the same ExecutionMode that backs Claude's --allowedTools.
type CrushGuardPolicy struct {
	// WriteRoots are absolute directory prefixes a file-writing tool
	// may target. Empty means no writes at all.
	WriteRoots []string `json:"write_roots"`
	// AllowBash permits the shell tool. When false the shell is denied
	// outright, because a shell command trivially bypasses WriteRoots.
	AllowBash bool `json:"allow_bash"`
}

// NewCrushGuardPolicy projects an ExecutionMode onto the crush guard,
// resolving the mode's write globs against workDir into absolute
// write roots (see absoluteGlobRoot).
func NewCrushGuardPolicy(mode ExecutionMode, workDir string) CrushGuardPolicy {
	globs := mode.WriteGlobs()
	roots := make([]string, 0, len(globs))

	for _, glob := range globs {
		root := absoluteGlobRoot(glob, workDir)
		if root != "" && !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}

	return CrushGuardPolicy{WriteRoots: roots, AllowBash: mode.AllowsBash()}
}

// Encode renders the policy as the environment entry crush passes down
// to the hook.
func (p CrushGuardPolicy) Encode() (string, error) {
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to encode crush guard policy: %w", err)
	}

	return CrushPolicyEnvVar + "=" + string(encoded), nil
}

// LoadCrushGuardPolicy reads the policy the provider exported. A
// missing or malformed policy is an error, so a stray `crush run`
// outside the engine fails CLOSED rather than running unguarded.
func LoadCrushGuardPolicy() (CrushGuardPolicy, error) {
	raw := strings.TrimSpace(os.Getenv(CrushPolicyEnvVar))
	if raw == "" {
		return CrushGuardPolicy{}, pkgerrors.ErrCrushPolicyMissing
	}

	var policy CrushGuardPolicy

	err := json.Unmarshal([]byte(raw), &policy)
	if err != nil {
		return CrushGuardPolicy{}, pkgerrors.ErrCrushPolicyInvalid
	}

	return policy, nil
}

// Decide returns whether crush may run the tool, and why not when it
// may not. toolName is crush's lowercase tool id; targetPath is the
// file a write tool wants to touch (empty for non-file tools).
func (p CrushGuardPolicy) Decide(toolName, targetPath string) (bool, string) {
	name := strings.ToLower(strings.TrimSpace(toolName))

	if isCrushReadOnlyTool(name) {
		return true, ""
	}

	if name == "bash" {
		if p.AllowBash {
			return true, ""
		}

		return false, "shell is denied in this mode (it would bypass the write-root rule)"
	}

	if !slices.Contains(crushFileTools(), name) {
		return false, "tool " + name + " is not on the allowlist (unknown tools are treated as writers)"
	}

	return p.allowsWriteTo(targetPath)
}

// allowsWriteTo checks a target against the write roots after resolving
// symlinks on both sides: a lexical-only check would let a symlink inside
// a granted root (`<tmp>/link` → `/etc`) pass the prefix test and escape.
func (p CrushGuardPolicy) allowsWriteTo(targetPath string) (bool, string) {
	if len(p.WriteRoots) == 0 {
		return false, "this mode grants no write roots"
	}

	if strings.TrimSpace(targetPath) == "" {
		return false, "write tool did not name a target path"
	}

	if !filepath.IsAbs(targetPath) {
		return false, "write target must be an absolute path, got " + targetPath
	}

	resolved := resolveThroughSymlinks(filepath.Clean(targetPath))

	for _, root := range p.WriteRoots {
		if pathWithinRoot(resolved, resolveThroughSymlinks(root)) {
			return true, ""
		}
	}

	return false, "write target " + resolved + " is outside " + strings.Join(p.WriteRoots, ", ")
}

// resolveThroughSymlinks resolves the deepest existing ancestor and
// re-appends what doesn't exist: a write target usually doesn't exist
// yet, so plain EvalSymlinks on the full path would fail outright.
func resolveThroughSymlinks(path string) string {
	remainder := ""
	current := filepath.Clean(path)

	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, remainder)
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached the root without finding anything that exists;
			// the lexical form is all there is.
			return filepath.Clean(path)
		}

		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

// pathWithinRoot reports whether target sits inside root, comparing on
// path-component boundaries. A raw prefix test would accept
// `/repo/tmpfoo` for the root `/repo/tmp`.
func pathWithinRoot(target, root string) bool {
	root = strings.TrimSuffix(filepath.Clean(root), string(os.PathSeparator))
	if target == root {
		return true
	}

	return strings.HasPrefix(target, root+string(os.PathSeparator))
}

// isCrushReadOnlyTool reports whether the tool can only read.
func isCrushReadOnlyTool(name string) bool {
	if slices.Contains(crushReadOnlyTools(), name) {
		return true
	}

	for _, prefix := range crushReadOnlyToolPrefixes() {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// absoluteGlobRoot reduces a glob to the absolute directory prefix it can
// never escape (everything before the first wildcard), so `/repo/tmp/`
// can't prefix-match a sibling like `/repo/tmpfoo`.
func absoluteGlobRoot(glob, workDir string) string {
	fixed := glob

	if idx := strings.IndexAny(fixed, "*?["); idx >= 0 {
		fixed = fixed[:idx]
	}

	fixed = strings.TrimSpace(fixed)
	if fixed == "" {
		return ""
	}

	if !filepath.IsAbs(fixed) {
		fixed = filepath.Join(workDir, fixed)
	}

	return filepath.Clean(fixed) + string(os.PathSeparator)
}
