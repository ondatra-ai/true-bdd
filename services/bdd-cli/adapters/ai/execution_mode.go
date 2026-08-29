package ai

import (
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
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

// ExecutionMode is the domain's, not this package's: ports/ai_port.go names
// it, and an interface cannot be declared in terms of its own implementation.
type ExecutionMode = provider.ExecutionMode

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
