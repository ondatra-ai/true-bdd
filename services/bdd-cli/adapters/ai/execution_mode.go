package ai

import (
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// Tool specs shared by the built-in modes. Named because the same
// literals appear in both ThinkMode and EditMode.
const (
	bashToolName     = "Bash"
	readAnyToolSpec  = "Read(**)"
	globAnyToolSpec  = "Glob(**)"
	grepAnyToolSpec  = "Grep(**)"
	editAnyToolSpec  = "Edit(**)"
	multiEditAnySpec = "MultiEdit(**)"
	agentToolName    = "Agent"
	taskToolName     = "Task"
)

// ExecutionMode defines tool permissions for AI execution.
type ExecutionMode struct {
	AllowedTools    []string
	DisallowedTools []string
	// SourceWriteRoots are project trees outside tmp this mode opens for
	// writing (set only by GetSourceEditMode). Kept apart from AllowedTools:
	// codex has no sandbox level between scratch-only and the whole workspace, so merging them would over-grant it.
	SourceWriteRoots []string
}

// GrantsSourceWrites reports whether this mode opens any project tree
// outside tmp for writing.
func (m ExecutionMode) GrantsSourceWrites() bool {
	return len(m.SourceWriteRoots) > 0
}

// writeToolNames lists the tools that create or modify files. Used to
// project the Claude-style tool specs onto the coarser sandboxes of
// the crush and codex CLIs.
func writeToolNames() []string {
	return []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}
}

// WriteGlobs returns the path patterns this mode lets file-writing tools
// touch, parsed out of the Claude tool specs (`Write(./tmp/**)` →
// `./tmp/**`). Empty means the turn may not write at all; crush and codex derive their sandbox from this.
func (m ExecutionMode) WriteGlobs() []string {
	globs := make([]string, 0, len(m.AllowedTools))

	for _, spec := range m.AllowedTools {
		name, glob, ok := splitToolSpec(spec)
		if !ok || !slices.Contains(writeToolNames(), name) {
			continue
		}

		if !slices.Contains(globs, glob) {
			globs = append(globs, glob)
		}
	}

	return globs
}

// AllowsBash reports whether the mode permits shell execution.
func (m ExecutionMode) AllowsBash() bool {
	return !slices.Contains(m.DisallowedTools, bashToolName)
}

// splitToolSpec splits `Write(./tmp/**)` into ("Write", "./tmp/**").
// A bare tool name (`Bash`) has no pattern and reports false.
func splitToolSpec(spec string) (string, string, bool) {
	open := strings.Index(spec, "(")
	if open <= 0 || !strings.HasSuffix(spec, ")") {
		return "", "", false
	}

	return spec[:open], spec[open+1 : len(spec)-1], true
}

// ModeFactory creates execution modes with configured paths.
type ModeFactory struct {
	config *config.ViperConfig
}

// NewModeFactory creates a new mode factory.
func NewModeFactory(config *config.ViperConfig) *ModeFactory {
	return &ModeFactory{config: config}
}

// GetThinkMode returns ThinkMode with configured paths.
func (f *ModeFactory) GetThinkMode() ExecutionMode {
	tmpGlob := f.config.GetString("paths.tmp_glob")

	return ExecutionMode{
		AllowedTools: []string{
			readAnyToolSpec,
			"Write(" + tmpGlob + ")",
			globAnyToolSpec,
			grepAnyToolSpec,
		},
		DisallowedTools: []string{
			bashToolName,
			editAnyToolSpec,
			multiEditAnySpec,
			// Sub-agent tools: every prompt here is a single-turn `claude -p`
			// call, and delegating to a sub-agent ends the turn with no output
			// (the parent can't resume) — silently yielding an empty fix prompt.
			agentToolName,
			taskToolName,
		},
	}
}

// GetEditMode extends ThinkMode with Edit/MultiEdit against the tmp glob,
// for callers whose F: handlers mutate the scratch registry in place (e.g.
// us apply) rather than emit FILE_START/FILE_END markers.
func (f *ModeFactory) GetEditMode() ExecutionMode {
	tmpGlob := f.config.GetString("paths.tmp_glob")

	return ExecutionMode{
		AllowedTools: []string{
			readAnyToolSpec,
			"Write(" + tmpGlob + ")",
			"Edit(" + tmpGlob + ")",
			"MultiEdit(" + tmpGlob + ")",
			globAnyToolSpec,
			grepAnyToolSpec,
		},
		DisallowedTools: []string{
			bashToolName,
			// See GetThinkMode: sub-agent delegation breaks single-turn
			// `claude -p` calls, so keep it disallowed here too.
			agentToolName,
			taskToolName,
		},
	}
}

// GetSourceEditMode extends GetEditMode with write access to project roots
// outside tmp — the trees a `build` applier is told by its system prompt to
// write into. Without this the guard denies every such write and the applier silently reports "could not apply".
func (f *ModeFactory) GetSourceEditMode(roots []string) ExecutionMode {
	mode := f.GetEditMode()

	for _, root := range roots {
		glob := sourceRootGlob(root)
		if glob == "" {
			continue
		}

		mode.AllowedTools = append(mode.AllowedTools,
			"Write("+glob+")",
			"Edit("+glob+")",
			"MultiEdit("+glob+")",
		)
		mode.SourceWriteRoots = append(mode.SourceWriteRoots, glob)
	}

	return mode
}

// sourceRootGlob turns a declared root (`services/frontend`, or an
// already-globbed `./tests/**`) into a recursive write glob. Roots come
// from architecture.yaml and host config, so both spellings show up.
func sourceRootGlob(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}

	if strings.ContainsAny(root, "*?[") {
		return root
	}

	return strings.TrimSuffix(root, "/") + "/**"
}
