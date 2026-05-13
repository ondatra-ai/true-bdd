package shared

// McpServerType represents the type of MCP server.
type McpServerType string

const (
	// McpServerTypeStdio represents a stdio-based MCP server.
	McpServerTypeStdio McpServerType = "stdio"
	// McpServerTypeSSE represents a Server-Sent Events MCP server.
	McpServerTypeSSE McpServerType = "sse"
	// McpServerTypeHTTP represents an HTTP-based MCP server.
	McpServerTypeHTTP McpServerType = "http"
)

// McpServerConfig represents MCP server configuration.
type McpServerConfig interface {
	GetType() McpServerType
}

// McpStdioServerConfig configures an MCP stdio server.
type McpStdioServerConfig struct {
	Type    McpServerType     `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// McpSSEServerConfig configures an MCP Server-Sent Events server.
type McpSSEServerConfig struct {
	Type    McpServerType     `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// McpHTTPServerConfig configures an MCP HTTP server.
type McpHTTPServerConfig struct {
	Type    McpServerType     `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}
