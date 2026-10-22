package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	defaultMCPPort   = 8080
	maxTCPPort       = 65535
	dialTimeout      = 2 * time.Second
	requestTimeout   = 10 * time.Second
	protocolVersion  = "2025-06-18"
	jsonRPCVersion   = "2.0"
	jsonRPCRequestID = 1
	bodySnippetLimit = 512
)

// ErrNoResponse is what a Then clause returns when no When step of the
// scenario recorded an exchange for it to assert on.
var ErrNoResponse = errors.New("no HTTP response recorded")

// Register binds every phrase of the mcp suite. One definition per
// phrase, each anchored and parameterised: add a new phrase here rather
// than in a second Register function.
func Register(suite *Suite) {
	suite.Step(`^the MCP server is running on (?:its configured port|port (\d+))$`, serverIsRunning)
	suite.Step(`^the (.+) posts an? (valid|malformed) JSON-RPC ([A-Za-z][A-Za-z0-9_/.-]*) request to (/\S*)$`, postJSONRPCRequest)
	suite.Step(`^the server returns HTTP (\d{3})$`, serverReturnsHTTPStatus)
}

// serverIsRunning prepares: it resolves the server's base URL and
// refuses to proceed unless something is listening there.
func serverIsRunning(s *State, args ...string) error {
	port, err := resolvePort(arg(args, 0))
	if err != nil {
		return err
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("expected the MCP server to be listening on %s, got: %w", addr, err)
	}

	defer func() { _ = conn.Close() }()

	s.Port = port
	s.BaseURL = "http://" + addr

	return nil
}

// resolvePort prefers the port the step named, then MCP_PORT, then the
// service's default.
func resolvePort(named string) (int, error) {
	raw := named
	if raw == "" {
		raw = os.Getenv("MCP_PORT")
	}

	if raw == "" {
		return defaultMCPPort, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("expected a TCP port number, got %q: %w", raw, err)
	}

	if port < 1 || port > maxTCPPort {
		return 0, fmt.Errorf("expected a TCP port in 1..%d, got %d", maxTCPPort, port)
	}

	return port, nil
}

// postJSONRPCRequest acts: it posts the envelope the step described and
// records the status and body for the Thens.
func postJSONRPCRequest(s *State, args ...string) error {
	role, validity, method, path := arg(args, 0), arg(args, 1), arg(args, 2), arg(args, 3)

	if s.BaseURL == "" {
		return errors.New("expected a resolved MCP server, got none: a Given step must establish it before a request is posted")
	}

	body, err := requestBody(role, validity, method)
	if err != nil {
		return err
	}

	s.RequestURL = s.BaseURL + path
	s.StatusCode = 0
	s.Body = nil

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.RequestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not build the POST to %s: %w", s.RequestURL, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("expected a response to the JSON-RPC %s request to %s, got: %w", method, s.RequestURL, err)
	}

	defer func() { _ = resp.Body.Close() }()

	read, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("expected to read the response from %s, got: %w", s.RequestURL, err)
	}

	s.StatusCode = resp.StatusCode
	s.Body = read

	return nil
}

// requestBody renders the JSON-RPC envelope the step asked for; the
// malformed variant is truncated on purpose, so the server rejects it.
func requestBody(role, validity, method string) ([]byte, error) {
	if validity == "malformed" {
		return []byte(`{"jsonrpc": "2.0", "method":`), nil
	}

	payload := map[string]any{
		"jsonrpc": jsonRPCVersion,
		"id":      jsonRPCRequestID,
		"method":  method,
		"params": map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": role, "version": "0.0.0"},
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("could not encode the JSON-RPC %s request: %w", method, err)
	}

	return encoded, nil
}

// serverReturnsHTTPStatus asserts: the recorded status is the one the
// step names, and the failure says which it got and what came back.
func serverReturnsHTTPStatus(s *State, args ...string) error {
	raw := arg(args, 0)

	want, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("expected an HTTP status code, got %q: %w", raw, err)
	}

	if s.StatusCode == 0 {
		return fmt.Errorf("%w: expected HTTP %d, but no When step of this scenario posted a request", ErrNoResponse, want)
	}

	if s.StatusCode != want {
		return fmt.Errorf(
			"expected HTTP %d from %s, got HTTP %d; body: %s",
			want, s.RequestURL, s.StatusCode, snippet(s.Body),
		)
	}

	return nil
}

func arg(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}

	return args[i]
}

func snippet(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}

	if len(body) > bodySnippetLimit {
		return string(body[:bodySnippetLimit]) + "...(truncated)"
	}

	return string(body)
}
