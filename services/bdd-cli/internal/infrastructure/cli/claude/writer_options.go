package claude

import "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/shell"

// WriterOptions enables only explicit settings, tools and working roots.
type WriterOptions struct {
	Settings  string
	ReadDirs  []string
	SessionID string
}

func (w WriterOptions) args() []string {
	const (
		baseArgs    = 11
		perReadRoot = 2
	)

	args := make([]string, 0, baseArgs+perReadRoot*len(w.ReadDirs))

	args = append(args, settingSourcesFlag, "", strictMCPFlag, "--restricted", "--disable-slash-commands",
		"--tools", "Read,Glob,Grep,Edit,Write,MultiEdit,Bash", "--settings", w.Settings, "--session-id", w.SessionID)
	for _, dir := range w.ReadDirs {
		args = append(args, "--add-dir", dir)
	}

	return args
}

func writerEnv(env shell.Env) shell.Env {
	return env.Strip("CLAUDE_CODE_SAFE_MODE", "CLAUDE_CODE_SIMPLE", "CLAUDE_RELAUNCH_SESSION_ADD_DIRS").Set(
		"BASH_DEFAULT_TIMEOUT_MS=600000", "BASH_MAX_TIMEOUT_MS=600000",
		"CLAUDE_CODE_DISABLE_CLAUDE_MDS=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
		"CLAUDE_CODE_DISABLE_BUNDLED_SKILLS=1", "CLAUDE_CODE_DISABLE_WORKFLOWS=1",
		"CLAUDE_CODE_DISABLE_OFFICIAL_MARKETPLACE_AUTOINSTALL=1", "CLAUDE_CODE_DISABLE_PLUGIN_FORWARDING=1",
		"CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1",
	)
}

func (o Options) isolationArgs() []string {
	if o.Writer != nil {
		return o.Writer.args()
	}

	return isolation(o.Minimal)
}
