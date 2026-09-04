//go:build bdd

// Package steps holds the step definitions for the mcp suite. Every
// Given/When/Then step of a scenario whose service is mcp-service binds
// to exactly one pattern registered in Register below.
package steps

import (
	"bytes"
	"context"
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

// State is the value every definition in this suite shares: one scenario
// gets one State, threaded from its Given steps through to its Thens.
type State struct {
	BaseURL    string
	StatusCode int
	Body       string
}

// Suite is the registrar the scenario runner hands to Register. It matches
// a step's text against the pattern and calls fn with the suite state
// followed by one string per capture group.
type Suite interface {
	Step(pattern string, fn any)
}

const (
	defaultPort   = 8080
	dialTimeout   = 5 * time.Second
	clientTimeout = 30 * time.Second
	bodySnippet   = 200
)

// Register binds every step this suite understands.
func Register(suite Suite) {
	suite.Step(`^the (.+) server is running on its configured port$`, serverIsRunning)
	suite.Step(`^the (.+) posts a valid JSON-RPC (\S+) request to (\S+)$`, postsJSONRPCRequest)
	suite.Step(`^the server returns HTTP (\d{3})$`, serverReturnsStatus)
}

// serverIsRunning is a Given: it resolves the port the named service is
// configured on and records the base URL once the port answers.
func serverIsRunning(s *State, service string) error {
	port, err := configuredPort(service)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("expected the %s server listening on %s, but dialing it failed: %w", service, addr, err)
	}

	defer func() { _ = conn.Close() }()

	s.BaseURL = "http://" + addr

	return nil
}

// configuredPort reads the port from <SERVICE>_PORT, which is how whatever
// booted the service tells the suite where it went.
func configuredPort(service string) (int, error) {
	key := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(service)) + "_PORT"

	raw, ok := os.LookupEnv(key)
	if !ok {
		return defaultPort, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("expected %s to hold a port number, got %q: %w", key, raw, err)
	}

	return port, nil
}

// postsJSONRPCRequest is a When: it posts a well-formed JSON-RPC request for
// the named method and records the response for the Then steps.
func postsJSONRPCRequest(s *State, actor, method, path string) error {
	if s.BaseURL == "" {
		return fmt.Errorf("%s cannot post to %s: no server address recorded — the Given step that starts the server has not run", actor, path)
	}

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("building the JSON-RPC %s request body: %w", method, err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, s.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building the POST to %s%s: %w", s.BaseURL, path, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{Timeout: clientTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("posting the JSON-RPC %s request to %s: %w", method, path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading the response to the JSON-RPC %s request: %w", method, err)
	}

	s.StatusCode = resp.StatusCode
	s.Body = string(raw)

	return nil
}

// serverReturnsStatus is a Then: it asserts the recorded status, naming what
// was expected and what arrived.
func serverReturnsStatus(s *State, want string) error {
	code, err := strconv.Atoi(want)
	if err != nil {
		return fmt.Errorf("expected a numeric HTTP status in the step text, got %q: %w", want, err)
	}

	if s.StatusCode == 0 {
		return fmt.Errorf("expected HTTP %d, but no response was recorded — no When step posted a request", code)
	}

	if s.StatusCode != code {
		return fmt.Errorf("expected HTTP %d, got %d (body: %s)", code, s.StatusCode, snippet(s.Body))
	}

	return nil
}

// snippet keeps a failure message readable when the body is large.
func snippet(body string) string {
	if body == "" {
		return "<empty>"
	}

	if len(body) > bodySnippet {
		return body[:bodySnippet] + "..."
	}

	return body
}
