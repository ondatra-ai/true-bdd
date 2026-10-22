//go:build bdd

// Package steps holds the step definitions for the `mcp` test suite.
//
// The suite is resolved from docs/architecture/architecture.yaml: service
// `mcp-service` owns the tests/mcp tree, so every definition the mcp
// scenarios bind to lives in this package and is registered from the one
// Register call below.
package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// StepFunc is the shape of every step definition in this package: it
// receives the pattern's capture groups in order and reports failure as an
// error, because that error IS the test failure a reader gets.
type StepFunc func(args ...string) error

// Suite is the seam the suite runner satisfies. It is declared here rather
// than imported so this package carries no dependency on the runner, which
// lets the runner import these definitions and call Register without an
// import cycle.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

const (
	// defaultPort is the configured MCP port when the environment is silent.
	defaultPort = 8080
	// portEnvVar overrides defaultPort for a run.
	portEnvVar = "MCP_PORT"

	dialTimeout  = 2 * time.Second
	httpTimeout  = 10 * time.Second
	maxBodyBytes = 64 << 10

	// protocolVersion is the MCP revision the suite advertises when it
	// initializes, so the request the When step sends is a valid one.
	protocolVersion = "2025-06-18"
)

// State is the single piece of mutable state the mcp step definitions
// share: a Given fills in the server coordinates, a When records the
// exchange it performed, a Then reads that exchange back.
type State struct {
	// BaseURL is the root the When steps post to, e.g. http://127.0.0.1:8080.
	BaseURL string
	// Port is the TCP port the Given step found the server listening on.
	Port int
	// PortSource names where Port came from, for error messages.
	PortSource string
	// LastRequest describes the exchange the most recent When performed.
	LastRequest string
	// Status is that exchange's HTTP status code; 0 when none has happened.
	Status int
	// Body is that exchange's response payload.
	Body []byte

	client *http.Client
}

// NewState returns the shared state with its HTTP client ready.
func NewState() *State {
	return &State{client: &http.Client{Timeout: httpTimeout}}
}

// Register binds the mcp suite's entire step vocabulary. Every definition
// the suite has is listed here.
func Register(suite Suite) {
	state := NewState()

	// Given: "the MCP server is running on its configured port", and the
	// same sentence naming a port explicitly ("... on port 8080").
	suite.Step(`^the MCP server is running on (?:its configured port|port (\d+))$`, func(args ...string) error {
		return state.serverIsRunning(arg(args, 0))
	})

	// When: any actor posting any valid JSON-RPC method to any endpoint.
	suite.Step(`^the (.+) posts a valid JSON-RPC ([A-Za-z][A-Za-z0-9_/.-]*) request to (/\S*)$`, func(args ...string) error {
		return state.postJSONRPC(arg(args, 0), arg(args, 1), arg(args, 2))
	})

	// Then: any HTTP status code.
	suite.Step(`^the server returns HTTP (\d{3})$`, func(args ...string) error {
		return state.statusIs(arg(args, 0))
	})
}

// serverIsRunning resolves the port the scenario means and proves something
// is accepting connections on it before any When step relies on it.
func (s *State) serverIsRunning(portArg string) error {
	port, source, err := resolvePort(portArg)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf(
			"expected the MCP server to be listening on %s (port from %s), but dialling it failed within %s: %v",
			addr, source, dialTimeout, err)
	}
	_ = conn.Close()

	s.Port = port
	s.PortSource = source
	s.BaseURL = "http://" + addr
	s.Status = 0
	s.Body = nil
	s.LastRequest = ""
	return nil
}

// resolvePort prefers a port named in the step text, then the environment,
// then the default, and says which it used.
func resolvePort(portArg string) (port int, source string, err error) {
	if portArg != "" {
		port, err = parsePort(portArg)
		if err != nil {
			return 0, "", fmt.Errorf("expected the port in the step text to be a number in 1..65535, got %q", portArg)
		}
		return port, "the step text", nil
	}

	raw, ok := os.LookupEnv(portEnvVar)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultPort, fmt.Sprintf("the default configured port (%s is unset)", portEnvVar), nil
	}
	port, err = parsePort(raw)
	if err != nil {
		return 0, "", fmt.Errorf("expected %s to hold a port number in 1..65535, got %q", portEnvVar, raw)
	}
	return port, portEnvVar, nil
}

func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of range", port)
	}
	return port, nil
}

// postJSONRPC sends a well-formed JSON-RPC request for the named method to
// the named endpoint and records the response for a Then step to assert on.
func (s *State) postJSONRPC(actor, method, path string) error {
	if s.BaseURL == "" {
		return fmt.Errorf(
			"cannot post the JSON-RPC %s request: no MCP server address is known yet — the Given step that finds the running server has not run",
			method)
	}

	payload, err := json.Marshal(jsonRPCRequest(method))
	if err != nil {
		return fmt.Errorf("expected to encode the JSON-RPC %s request, but marshalling it failed: %v", method, err)
	}

	url := strings.TrimSuffix(s.BaseURL, "/") + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("expected to build a POST to %s, but constructing the request failed: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	// MCP's streamable HTTP transport requires the client to accept both.
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf(
			"%s posted JSON-RPC %s to %s and expected a response, but the request failed: %v",
			describeActor(actor), method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	s.Status = resp.StatusCode
	s.Body = body
	s.LastRequest = fmt.Sprintf("POST %s (JSON-RPC %s, as %s)", url, method, describeActor(actor))
	if readErr != nil {
		return fmt.Errorf("%s: got HTTP %d but reading the response body failed: %v", s.LastRequest, resp.StatusCode, readErr)
	}
	return nil
}

// jsonRPCRequest builds a valid request envelope for method, including the
// parameters MCP requires for initialize.
func jsonRPCRequest(method string) map[string]any {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}
	if method == "initialize" {
		req["params"] = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "true-bdd-mcp-suite",
				"version": "1.0.0",
			},
		}
		return req
	}
	req["params"] = map[string]any{}
	return req
}

// statusIs asserts the recorded response carried the expected status code.
func (s *State) statusIs(want string) error {
	code, err := strconv.Atoi(want)
	if err != nil {
		return fmt.Errorf("expected the status code in the step text to be a number, got %q", want)
	}
	if s.Status == 0 {
		return fmt.Errorf("expected HTTP %d, but no response has been recorded — no When step posted a request", code)
	}
	if s.Status != code {
		return fmt.Errorf("expected HTTP %d from %s, got HTTP %d; response body: %s",
			code, s.LastRequest, s.Status, bodySnippet(s.Body))
	}
	return nil
}

// arg returns capture group i, or "" when the group did not participate in
// the match (an optional alternative, such as the explicit port).
func arg(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return strings.TrimSpace(args[i])
}

func describeActor(actor string) string {
	if actor == "" {
		return "the client"
	}
	return actor
}

func bodySnippet(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "<empty>"
	}
	const limit = 300
	if len(trimmed) > limit {
		return strconv.Quote(trimmed[:limit]) + " (truncated)"
	}
	return strconv.Quote(trimmed)
}
