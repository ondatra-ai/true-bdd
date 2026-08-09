package ai

import (
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
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
}

// writeToolNames lists the tools that create or modify files. Used to
// project the Claude-style tool specs onto the coarser sandboxes of
// the crush and codex CLIs.
func writeToolNames() []string {
	return []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}
}

// WriteGlobs returns the path patterns this mode lets file-writing
// tools touch, parsed out of the Claude tool specs
// (`Write(./tmp/**)` → `./tmp/**`). An empty result means the turn may
// not write at all.
//
// Providers with no native per-tool allowlist (crush, codex) derive
// their sandbox from this, so ExecutionMode stays the single source of
// truth for permissions across every CLI rather than each provider
// inventing its own rules.
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
		[]string{
			readAnyToolSpec,
			"Write(" + tmpGlob + ")",
			globAnyToolSpec,
			grepAnyToolSpec,
		},
		[]string{
			bashToolName,
			editAnyToolSpec,
			multiEditAnySpec,
			// Sub-agent tools. Every prompt here is a single-turn
			// `claude -p` call; delegating to a sub-agent and awaiting it
			// ends the turn with no output (the parent cannot resume),
			// which silently yields an empty fix prompt. Force inline work.
			agentToolName,
			taskToolName,
		},
	}
}

// GetEditMode returns a mode that additionally allows Edit and
// MultiEdit against the configured tmp glob, so callers whose F:
// handlers mutate the scratch registry in place (e.g. us apply) can
// actually run their prompts. ThinkMode disallows Edit globally,
// which is correct for handlers that emit FILE_START/FILE_END
// markers (us create / us refine), but wrong for us apply.
func (f *ModeFactory) GetEditMode() ExecutionMode {
	tmpGlob := f.config.GetString("paths.tmp_glob")

	return ExecutionMode{
		[]string{
			readAnyToolSpec,
			"Write(" + tmpGlob + ")",
			"Edit(" + tmpGlob + ")",
			"MultiEdit(" + tmpGlob + ")",
			globAnyToolSpec,
			grepAnyToolSpec,
		},
		[]string{
			bashToolName,
			// See GetThinkMode: sub-agent delegation breaks single-turn
			// `claude -p` calls, so keep it disallowed here too.
			agentToolName,
			taskToolName,
		},
	}
}

// GetSourceEditMode extends GetEditMode with write access to project
// roots outside tmp — the production or test trees a `build` command's
// applier is supposed to author into.
//
// Without it those turns are told by their system prompt to write under
// `services/*`, while the only enforced write root is the tmp glob; the
// guard then denies every write and the applier reports back that it
// could not apply. Permissions live here rather than in the prompt
// because ExecutionMode is what the crush guard and codex sandbox are
// actually derived from.
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
