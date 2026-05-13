package shared

// Options configures the Claude Code SDK behavior.
type Options struct {
	// Tool Control
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// System Prompts & Model
	SystemPrompt       *string `json:"system_prompt,omitempty"`
	AppendSystemPrompt *string `json:"append_system_prompt,omitempty"`
	Model              *string `json:"model,omitempty"`
	MaxThinkingTokens  int     `json:"max_thinking_tokens,omitempty"`

	// Permission & Safety System
	PermissionMode           *PermissionMode `json:"permission_mode,omitempty"`
	PermissionPromptToolName *string         `json:"permission_prompt_tool_name,omitempty"`

	// Session & State Management
	ContinueConversation bool    `json:"continue_conversation,omitempty"`
	Resume               *string `json:"resume,omitempty"`
	MaxTurns             int     `json:"max_turns,omitempty"`
	Settings             *string `json:"settings,omitempty"`

	// File System & Context
	Cwd     *string  `json:"cwd,omitempty"`
	AddDirs []string `json:"add_dirs,omitempty"`

	// MCP Integration
	McpServers map[string]McpServerConfig `json:"mcp_servers,omitempty"`

	// Extensibility
	ExtraArgs map[string]*string `json:"extra_args,omitempty"`

	// CLI Path (for testing and custom installations)
	CLIPath *string `json:"cli_path,omitempty"`
}
