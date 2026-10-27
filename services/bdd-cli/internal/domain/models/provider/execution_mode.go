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
	LibraryReadRoots []string
	AllowedTools     []string
	DisallowedTools  []string
	// SourceWriteRoots are project trees outside tmp this mode opens for
	// writing (set by ModeFactory); tool patterns also carry scratch and exact file grants.
	SourceWriteRoots []string
	// TestCommand is a restricted replay grant consumed by the explicit Claude build-code writer.
	TestCommand *TestCommand
}

// GrantsSourceWrites reports whether this mode opens any project tree
// outside tmp for writing.
func (m ExecutionMode) GrantsSourceWrites() bool {
	return len(m.SourceWriteRoots) > 0
}

// writeToolNames lists the tools that create or modify files. Used to
// project Claude-style tool specs onto the codex apply permission profile.
func writeToolNames() []string {
	return []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}
}

// WriteGlobs returns the path patterns this mode lets file-writing tools
// touch, parsed out of the Claude tool specs (`Write(./tmp/**)` →
// `./tmp/**`). Codex apply profiles consume these patterns; role independently keeps plans read-only.
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
