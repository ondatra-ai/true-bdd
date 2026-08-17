//go:build bdd

// Package steps holds the step definitions for the `mcp` test suite
// (service mcp-service, rooted at tests/mcp per
// docs/architecture/architecture.yaml). `build tests` resolves each
// registry scenario's Given/When/Then steps against the patterns
// registered in Register below.
package steps

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// State is the shared state every step definition in the mcp suite
// operates on. A Given prepares it, a When records the HTTP response it
// produced, and a Then asserts against that response.
type State struct {
	// BaseURL is the origin of the running MCP server, e.g.
	// "http://127.0.0.1:8080".
	BaseURL string
	// Response is the most recent HTTP response captured by a When step.
	Response *http.Response
}

// StepFunc is the shape of every step definition: it receives the shared
// State and the regexp capture groups of the matched step, and returns a
// non-nil error naming what it expected and what it got when the step
// fails.
type StepFunc func(s *State, args ...string) error

// Suite is the subset of the mcp runner (tests/mcp/scenarios) that step
// definitions bind against. It is declared here so this package never
// imports the runner and the two cannot form an import cycle.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// Register binds every step definition the mcp suite owns. Every future
// scenario assigned to this suite adds its definitions here, so one call
// site still lists everything the suite binds.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running on its configured port$`, givenServerRunning)
	suite.Step(`^the Claude User posts a valid JSON-RPC (\w+) request to (\S+)$`, whenPostJSONRPC)
	suite.Step(`^the server returns HTTP (\d+)$`, thenServerReturnsStatus)
}

// givenServerRunning confirms the MCP server is up on its configured port
// and records its base URL for the When step that follows.
func givenServerRunning(s *State, _ ...string) error {
	if s.BaseURL == "" {
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "8080"
		}
		s.BaseURL = "http://127.0.0.1:" + port
	}
	resp, err := http.Get(s.BaseURL + "/healthz")
	if err != nil {
		return fmt.Errorf("MCP server is not running at %s: %w", s.BaseURL, err)
	}
	_ = resp.Body.Close()
	return nil
}

// whenPostJSONRPC posts a valid JSON-RPC request of the captured method
// to the captured path and records the response on the state.
func whenPostJSONRPC(s *State, args ...string) error {
	if len(args) < 2 {
		return fmt.Errorf("expected a JSON-RPC method and a path, got %v", args)
	}
	method, path := args[0], args[1]
	body := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":%q,"params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude","version":"1.0"}}}`,
		method,
	)
	resp, err := http.Post(s.BaseURL+path, "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST %s%s (%s) failed: %w", s.BaseURL, path, method, err)
	}
	s.Response = resp
	return nil
}

// thenServerReturnsStatus asserts the recorded response carried the
// captured HTTP status code.
func thenServerReturnsStatus(s *State, args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected an HTTP status code in the step, got %v", args)
	}
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("HTTP status %q is not a number: %w", args[0], err)
	}
	if s.Response == nil {
		return fmt.Errorf("expected HTTP %d but no response was captured; a When step must run first", want)
	}
	defer func() { _ = s.Response.Body.Close() }()
	if s.Response.StatusCode != want {
		snippet, _ := io.ReadAll(io.LimitReader(s.Response.Body, 512))
		return fmt.Errorf("expected HTTP %d, got %d (body: %s)", want, s.Response.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
