package steps

// Step definitions for the `mcp-service` suite.
//
// Every definition is anchored and parameterised: the server name, the
// acting user, the JSON-RPC method, the request path and the expected
// status code are captures, so one definition serves every registry
// scenario phrased that way. Assertions name what was expected and what
// actually happened, because that error IS the failure a reader gets.

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

// Register wires this suite's vocabulary into the harness. It is the
// suite's single registration entry point: add new definitions here.
func Register(suite Suite) {
	suite.Step(`^the (.+?) server is running on its configured port$`, givenServerRunningOnConfiguredPort)
	suite.Step(`^the (.+?) posts a valid JSON-RPC (\S+) request to (/\S*)$`, whenPostsValidJSONRPCRequest)
	suite.Step(`^the server returns HTTP (\d+)$`, thenServerReturnsHTTPStatus)
}

const (
	defaultClientTimeout = 10 * time.Second
	defaultDialTimeout   = 2 * time.Second
	protocolVersion      = "2026-10-22"
)

// givenServerRunningOnConfiguredPort prepares: it resolves the suite's
// base URL against the configured port and proves the listener is up.
func givenServerRunningOnConfiguredPort(s *State, args []string) error {
	captures := tail(args, 1)
	name := "server"
	if len(captures) == 1 && strings.TrimSpace(captures[0]) != "" {
		name = strings.TrimSpace(captures[0]) + " server"
	}
	if s == nil {
		return fmt.Errorf("expected scenario state for the %s, got nil", name)
	}

	port := os.Getenv("MCP_PORT")
	if port == "" {
		return fmt.Errorf("expected the %s to have a configured port in MCP_PORT, got an empty value", name)
	}
	if s.Client == nil {
		s.Client = &http.Client{Timeout: defaultClientTimeout}
	}
	if s.BaseURL == "" {
		s.BaseURL = "http://127.0.0.1:" + port
	}

	address, err := url.Parse(s.BaseURL)
	if err != nil {
		return fmt.Errorf("expected the %s base URL %q to parse, got error: %w", name, s.BaseURL, err)
	}
	if address.Port() != port {
		return fmt.Errorf("expected the %s to listen on its configured port %s, got base URL %q on port %q", name, port, s.BaseURL, address.Port())
	}

	host := address.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), defaultDialTimeout)
	if err != nil {
		return fmt.Errorf("expected the %s to be running on configured port %s, got dial error: %w", name, port, err)
	}
	_ = conn.Close()
	return nil
}

// whenPostsValidJSONRPCRequest acts: it POSTs a well-formed JSON-RPC 2.0
// envelope for the captured method to the captured path and records the
// response on the state for the Then steps.
func whenPostsValidJSONRPCRequest(s *State, args []string) error {
	captures := tail(args, 3)
	if len(captures) < 3 {
		return fmt.Errorf("expected 3 captures (actor, JSON-RPC method, request path), got %d: %v", len(captures), captures)
	}
	actor, method, path := captures[0], captures[1], captures[2]

	if s == nil {
		return fmt.Errorf("expected scenario state before %s posts %s, got nil", actor, method)
	}
	if s.BaseURL == "" {
		return fmt.Errorf("expected a base URL before %s posts a JSON-RPC %s request to %s, got an empty BaseURL (is the Given step registered?)", actor, method, path)
	}
	if s.Client == nil {
		s.Client = &http.Client{Timeout: defaultClientTimeout}
	}

	params := "{}"
	if strings.EqualFold(method, "initialize") {
		params = fmt.Sprintf(`{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":%q,"version":"1.0.0"}}`, protocolVersion, actor)
	}
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)

	endpoint := strings.TrimSuffix(s.BaseURL, "/") + path
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("expected to build a POST request for %s, got error: %w", endpoint, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	if s.Response != nil && s.Response.Body != nil {
		_ = s.Response.Body.Close()
	}
	s.Response = nil

	response, err := s.Client.Do(request)
	if err != nil {
		return fmt.Errorf("expected POST %s with a valid JSON-RPC %s request to reach the server, got transport error: %w", endpoint, method, err)
	}

	raw, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		return fmt.Errorf("expected to read the response body of POST %s, got error: %w", endpoint, err)
	}
	// Re-arm the body so later assertions can still read it.
	response.Body = io.NopCloser(bytes.NewReader(raw))
	s.Response = response
	return nil
}

// thenServerReturnsHTTPStatus asserts the recorded response's status code.
func thenServerReturnsHTTPStatus(s *State, args []string) error {
	captures := tail(args, 1)
	if len(captures) < 1 {
		return fmt.Errorf("expected 1 capture (the HTTP status code), got %d: %v", len(captures), captures)
	}
	want, err := strconv.Atoi(strings.TrimSpace(captures[0]))
	if err != nil {
		return fmt.Errorf("expected a numeric HTTP status in the step, got %q: %w", captures[0], err)
	}
	if s == nil || s.Response == nil {
		return fmt.Errorf("expected HTTP %d, got no response at all: no request was recorded on the scenario state", want)
	}
	if s.Response.StatusCode != want {
		return fmt.Errorf("expected HTTP %d, got HTTP %d (%q)%s", want, s.Response.StatusCode, s.Response.Status, bodySnippet(s.Response))
	}
	return nil
}

// tail returns the last n entries of args. Harnesses differ on whether
// the full regexp match is prepended to the captures; taking the tail is
// correct under either convention.
func tail(args []string, n int) []string {
	if n <= 0 || len(args) <= n {
		return args
	}
	return args[len(args)-n:]
}

// bodySnippet renders a short, re-readable excerpt of a response body for
// failure messages. It restores the body so callers can read it again.
func bodySnippet(response *http.Response) string {
	if response == nil || response.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return ""
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 200 {
		trimmed = trimmed[:200] + "…"
	}
	return "; body: " + trimmed
}
