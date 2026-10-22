//go:build bdd

package steps

import (
	"bytes"
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
	portEnvVar           = "MCP_PORT"
	configuredPortPhrase = "its configured port"
	defaultPort          = 8080
	maxPort              = 65535
	localHost            = "127.0.0.1"
	bootBudget           = 5 * time.Second
	dialTimeout          = 2 * time.Second
	pollInterval         = 100 * time.Millisecond
	requestTimeout       = 10 * time.Second
	maxBodyBytes         = 1 << 20
	bodyReportLimit      = 512
	initializeMethod     = "initialize"
	protocolVersion      = "2025-06-18"
)

// State is the one value every definition in this package reads and
// writes: the harness creates it per scenario and hands it to Register.
type State struct {
	Port       int
	BaseURL    string
	LastActor  string
	LastMethod string
	LastPath   string
	LastStatus int
	LastBody   string

	client *http.Client
}

// NewState returns the state a single scenario runs against.
func NewState() *State {
	return &State{client: &http.Client{Timeout: requestTimeout}}
}

// configuredPort resolves the port this suite was told to drive.
func configuredPort() (int, error) {
	raw := os.Getenv(portEnvVar)
	if raw == "" {
		return defaultPort, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("expected %s to hold a TCP port, got %q: %w", portEnvVar, raw, err)
	}

	if port <= 0 || port > maxPort {
		return 0, fmt.Errorf("expected %s to hold a TCP port in 1..%d, got %d", portEnvVar, maxPort, port)
	}

	return port, nil
}

// waitForListener dials the port until the server answers or the budget runs out.
func waitForListener(port int, budget time.Duration) error {
	addr := net.JoinHostPort(localHost, strconv.Itoa(port))
	deadline := time.Now().Add(budget)

	var lastErr error

	for {
		conn, err := net.DialTimeout("tcp", addr, dialTimeout)
		if err == nil {
			_ = conn.Close()

			return nil
		}

		lastErr = err

		if time.Now().After(deadline) {
			return fmt.Errorf("expected the MCP server to be accepting connections on %s within %s, last dial failed: %w", addr, budget, lastErr)
		}

		time.Sleep(pollInterval)
	}
}

// postJSONRPC sends one JSON-RPC request and records the response on the state.
func (s *State) postJSONRPC(actor, path, method string, valid bool) error {
	if s.BaseURL == "" {
		return errors.New("expected a running MCP server to post to, but no Given step established one")
	}

	body, err := jsonRPCBody(method, valid)
	if err != nil {
		return err
	}

	url := s.BaseURL + path

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("expected %s's POST %s to reach the MCP server, the request failed: %w", actor, url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("expected to read the response body of POST %s (HTTP %d), could not: %w", url, resp.StatusCode, err)
	}

	s.LastActor = actor
	s.LastMethod = method
	s.LastPath = path
	s.LastStatus = resp.StatusCode
	s.LastBody = string(raw)

	return nil
}

// jsonRPCBody renders the request body, deliberately truncated when the
// step asks for a malformed one.
func jsonRPCBody(method string, valid bool) ([]byte, error) {
	if !valid {
		return []byte(`{"jsonrpc": "2.0", "id": 1, "method":`), nil
	}

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}

	if method == initializeMethod {
		req["params"] = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "true-bdd-suite", "version": "0.0.0"},
		}
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("expected to encode a JSON-RPC %q request, could not: %w", method, err)
	}

	return raw, nil
}

// reportBody keeps a failure message readable without hiding the payload.
func reportBody(body string) string {
	if body == "" {
		return "<empty>"
	}

	if len(body) <= bodyReportLimit {
		return body
	}

	return body[:bodyReportLimit] + "… (truncated)"
}
