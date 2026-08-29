package provider

import (
	"slices"
	"strings"
)

// bashToolName is the one tool spec this type reasons about by name.
const bashToolName = "Bash"

// ExecutionMode defines tool permissions for AI execution. It lives in the
// domain because a port names it: the interface an adapter satisfies cannot
// be declared in terms of that adapter's own package.
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
