package steps

import "net/http"

// State is the per-scenario state the mcp suite keeps: the base URL the
// MCP server is reachable at (prepared by the Given step), and the most
// recent HTTP response the When step produced for the Then step to
// assert against.
type State struct {
	BaseURL  string
	Response *http.Response
}
