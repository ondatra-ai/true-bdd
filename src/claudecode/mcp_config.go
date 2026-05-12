package claudecode

import (
	"bdd-cli/src/claudecode/internal/shared"
)

// McpServerType defines the type of MCP server.
type McpServerType = shared.McpServerType

// McpServerConfig represents an MCP server configuration.
type McpServerConfig = shared.McpServerConfig

// McpStdioServerConfig represents a stdio MCP server configuration.
type McpStdioServerConfig = shared.McpStdioServerConfig

// McpSSEServerConfig represents an SSE MCP server configuration.
type McpSSEServerConfig = shared.McpSSEServerConfig

// McpHTTPServerConfig represents an HTTP MCP server configuration.
type McpHTTPServerConfig = shared.McpHTTPServerConfig

// Re-export MCP server type constants.
const (
	McpServerTypeStdio = shared.McpServerTypeStdio
	McpServerTypeSSE   = shared.McpServerTypeSSE
	McpServerTypeHTTP  = shared.McpServerTypeHTTP
)
