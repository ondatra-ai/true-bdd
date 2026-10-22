package steps

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Register binds the registry's step vocabulary for the mcp-service suite.
//
// One definition per distinct wording, anchored so a pattern cannot match a
// step it was not written for, and parameterised so a scenario naming a
// different port, JSON-RPC method, request path or status code reuses the
// definition it already has.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running on (?:its configured port|port "?([0-9]+)"?)$`, givenMCPServerRunning)
	suite.Step(`^the Claude User posts a valid JSON-RPC "?([A-Za-z][A-Za-z0-9/._-]*)"? request to "?(/[^"\s]*)"?$`, whenClaudeUserPostsJSONRPC)
	suite.Step(`^the server returns HTTP ([0-9]{3})$`, thenServerReturnsStatus)
}

// givenMCPServerRunning prepares the scenario against the MCP server the suite
// has already started: it confirms the host state carries a usable base URL,
// that the URL's port is the configured one (the step's own capture when it
// names a port, otherwise MCP_PORT), and that something is really listening
// there before any request is made.
func givenMCPServerRunning(state *State, args []string) error {
	if state == nil {
		return fmt.Errorf("the MCP server is running on its configured port: expected scenario state, got nil")
	}
	base := strings.TrimSpace(state.BaseURL)
	if base == "" {
		return fmt.Errorf("the MCP server is running on its configured port: expected the suite to supply State.BaseURL for the MCP server, got an empty string")
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("the MCP server is running on its configured port: expected State.BaseURL to be a URL, got %q: %w", base, err)
	}
	if state.Client == nil {
		state.Client = &http.Client{Timeout: 10 * time.Second}
	}

	wantPort := firstNumber(args)
	if wantPort == "" {
		wantPort = strings.TrimSpace(os.Getenv("MCP_PORT"))
	}
	if wantPort == "" {
		return fmt.Errorf("the MCP server is running on its configured port: expected the configured port in MCP_PORT, got an empty value")
	}
	if gotPort := endpoint.Port(); gotPort != wantPort {
		return fmt.Errorf("the MCP server is running on its configured port: expected the server on port %s, got base URL %q (port %q)", wantPort, base, gotPort)
	}

	conn, err := net.DialTimeout("tcp", endpoint.Host, 5*time.Second)
	if err != nil {
		return fmt.Errorf("the MCP server is running on its configured port: expected a listener on %s, got dial error: %w", endpoint.Host, err)
	}
	_ = conn.Close()

	// A fresh scenario must not inherit a previous response.
	state.Response = nil
	return nil
}

// whenClaudeUserPostsJSONRPC posts a well-formed JSON-RPC request for the
// captured method to the captured path on the suite's MCP server and records
// the response on the shared state for the Then steps to assert against.
func whenClaudeUserPostsJSONRPC(state *State, args []string) error {
	if state == nil {
		return fmt.Errorf("the Claude User posts a valid JSON-RPC request: expected scenario state, got nil")
	}
	method := firstWord(args)
	path := firstPath(args)
	if method == "" || path == "" {
		return fmt.Errorf("the Claude User posts a valid JSON-RPC request: expected a JSON-RPC method and a request path captured from the step, got method %q and path %q (captures %q)", method, path, args)
	}
	base := strings.TrimSpace(state.BaseURL)
	if base == "" {
		return fmt.Errorf("the Claude User posts a valid JSON-RPC %s request to %s: expected State.BaseURL to name the running MCP server, got an empty string", method, path)
	}
	if state.Client == nil {
		state.Client = &http.Client{Timeout: 10 * time.Second}
	}

	target := strings.TrimSuffix(base, "/") + path
	payload := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":%q,"params":{"protocolVersion":"2026-10-22","capabilities":{},"clientInfo":{"name":"claude-user","version":"1.0.0"}}}`,
		method,
	)

	request, err := http.NewRequest(http.MethodPost, target, bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("the Claude User posts a valid JSON-RPC %s request to %s: expected to build a POST for %q, got: %w", method, path, target, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")

	response, err := state.Client.Do(request)
	if err != nil {
		return fmt.Errorf("the Claude User posts a valid JSON-RPC %s request to %s: expected POST %s to reach the MCP server, got transport error: %w", method, path, target, err)
	}

	// Drain and re-wrap the body so the connection is released here while a
	// later assertion can still read what the server sent.
	raw, readErr := io.ReadAll(response.Body)
	if closeErr := response.Body.Close(); closeErr != nil && readErr == nil {
		readErr = closeErr
	}
	if readErr != nil {
		return fmt.Errorf("the Claude User posts a valid JSON-RPC %s request to %s: expected to read the response body of POST %s, got: %w", method, path, target, readErr)
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))

	state.Response = response
	return nil
}

// thenServerReturnsStatus asserts the recorded response carries the status code
// the step names.
func thenServerReturnsStatus(state *State, args []string) error {
	if state == nil {
		return fmt.Errorf("the server returns HTTP <status>: expected scenario state, got nil")
	}
	wanted := firstNumber(args)
	if wanted == "" {
		return fmt.Errorf("the server returns HTTP <status>: expected a numeric status code captured from the step, got captures %q", args)
	}
	want, err := strconv.Atoi(wanted)
	if err != nil {
		return fmt.Errorf("the server returns HTTP %s: expected the captured status to be a number, got %q: %w", wanted, wanted, err)
	}
	if state.Response == nil {
		return fmt.Errorf("the server returns HTTP %d: expected a recorded HTTP response, got none — a When step must send a request before this assertion", want)
	}
	if got := state.Response.StatusCode; got != want {
		return fmt.Errorf("the server returns HTTP %d: expected status %d, got %d (%q)", want, want, got, state.Response.Status)
	}
	return nil
}

// --- capture helpers -------------------------------------------------------
//
// The harness hands a definition its captures as a []string. These helpers
// pick a capture out by shape rather than by index, so a non-participating
// optional group or a leading full-match element cannot shift the meaning of a
// positional read. A full step line always contains spaces, so it never passes
// any of these shape checks.

// firstNumber returns the first capture that is a bare run of ASCII digits.
func firstNumber(args []string) string {
	for _, arg := range args {
		candidate := strings.Trim(strings.TrimSpace(arg), `"`)
		if digitsOnly(candidate) {
			return candidate
		}
	}
	return ""
}

// firstPath returns the first capture that looks like a request path.
func firstPath(args []string) string {
	for _, arg := range args {
		candidate := strings.Trim(strings.TrimSpace(arg), `"`)
		if strings.HasPrefix(candidate, "/") && !strings.ContainsAny(candidate, " \t") {
			return candidate
		}
	}
	return ""
}

// firstWord returns the first capture that is a single bare token and not a
// request path — the JSON-RPC method name.
func firstWord(args []string) string {
	for _, arg := range args {
		candidate := strings.Trim(strings.TrimSpace(arg), `"`)
		if candidate == "" || strings.HasPrefix(candidate, "/") {
			continue
		}
		if strings.ContainsAny(candidate, " \t") {
			continue
		}
		return candidate
	}
	return ""
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
