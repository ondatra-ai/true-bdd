//go:build bdd

// Step definitions for the `mcp` suite (service: mcp-service).
//
// First definitions file for this suite: it establishes the state the
// suite's steps share and the Register function listing everything the
// suite binds.
package steps

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort    = "8080"
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
	bodySnippetMax = 512
)

// State is the per-scenario state every definition in this suite shares:
// Given fills BaseURL, When fills Status and Body, Then reads them.
type State struct {
	BaseURL string
	Status  int
	Body    []byte
}

// Suite is the registrar the suite's runner hands to Register.
type Suite interface {
	Step(pattern string, fn func(s *State, args []string) error)
}

// Register lists every step definition the `mcp` suite binds.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running on its configured port$`, givenMCPServerRunning)
	suite.Step(`^the Claude User posts a valid JSON-RPC (\S+) request to (\S+)$`, whenPostJSONRPC)
	suite.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// givenMCPServerRunning resolves the configured port and proves something
// is listening on it before the scenario spends a request.
func givenMCPServerRunning(s *State, _ []string) error {
	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = defaultPort
	}

	address := net.JoinHostPort("127.0.0.1", port)

	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return fmt.Errorf("expected the MCP server listening on %s, got: %w", address, err)
	}

	_ = conn.Close()
	s.BaseURL = "http://" + address

	return nil
}

// whenPostJSONRPC posts one JSON-RPC request and records what came back.
func whenPostJSONRPC(s *State, args []string) error {
	method, endpoint := args[0], args[1]

	params := `{}`
	if method == "initialize" {
		params = `{"protocolVersion":"2024-11-05","capabilities":{},` +
			`"clientInfo":{"name":"true-bdd","version":"0.0.0"}}`
	}

	payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, s.BaseURL+endpoint, strings.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("building the JSON-RPC %s request to %s: %w", method, endpoint, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("posting the JSON-RPC %s request to %s: %w", method, endpoint, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading the response to the JSON-RPC %s request: %w", method, err)
	}

	s.Status = resp.StatusCode
	s.Body = body

	return nil
}

// thenServerReturnsStatus asserts the status the When step recorded.
func thenServerReturnsStatus(s *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("step names a non-numeric HTTP status %q: %w", args[0], err)
	}

	if s.Status == 0 {
		return fmt.Errorf("expected HTTP %d, but no response was recorded: no request was posted", want)
	}

	if s.Status != want {
		return fmt.Errorf("expected HTTP %d, got %d; body: %s", want, s.Status, snippet(s.Body))
	}

	return nil
}

// snippet keeps a failure message readable when the body is large.
func snippet(body []byte) string {
	if len(body) > bodySnippetMax {
		return string(body[:bodySnippetMax]) + "..."
	}

	return string(body)
}
