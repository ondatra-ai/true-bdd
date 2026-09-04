//go:build bdd

// Package steps holds this suite's step definitions: one
// suite.Step(regexp, func) per wording the registry's scenarios use.
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
	"time"
)

const (
	defaultPort    = 8080
	dialTimeout    = 5 * time.Second
	requestTimeout = 30 * time.Second
	bodyExcerpt    = 512
)

// Suite is the registration surface the harness hands to Register: one
// call per step definition, a regexp bound to the func that runs it.
type Suite interface {
	Step(pattern string, fn func(s *State, args []string) error)
}

// State is what this suite's steps share: a Given prepares it, a When
// acts and records into it, a Then asserts against it.
type State struct {
	BaseURL    string
	StatusCode int
	Body       []byte
}

// Register binds every wording this suite's scenarios use, so one call
// site still lists everything the suite binds.
func Register(suite Suite) {
	suite.Step(`^the (.+) server is running on (?:its configured port|port (\d+))$`, serverIsRunning)
	suite.Step(`^the (.+) posts a valid JSON-RPC ([A-Za-z0-9_./-]+) request to (\S+)$`, postsJSONRPCRequest)
	suite.Step(`^the server returns HTTP (\d{3})$`, serverReturnsStatus)
}

func serverIsRunning(s *State, args []string) error {
	name := arg(args, 0)

	port, err := resolvePort(arg(args, 1))
	if err != nil {
		return err
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("expected the %s server listening on %s, but nothing accepted a connection there: %w", name, addr, err)
	}

	_ = conn.Close()
	s.BaseURL = "http://" + addr

	return nil
}

func postsJSONRPCRequest(s *State, args []string) error {
	actor, method, path := arg(args, 0), arg(args, 1), arg(args, 2)
	if s.BaseURL == "" {
		return fmt.Errorf("expected a running server for %s to post to, but no Given step recorded one", actor)
	}

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  paramsFor(method, actor),
	})
	if err != nil {
		return fmt.Errorf("expected a marshalable JSON-RPC %s request: %w", method, err)
	}

	url := s.BaseURL + path

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("expected to build a POST to %s: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{Timeout: requestTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("expected a response from POST %s, got a transport error: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("expected to read the response body of POST %s: %w", url, err)
	}

	s.StatusCode = resp.StatusCode
	s.Body = payload

	return nil
}

func serverReturnsStatus(s *State, args []string) error {
	spoken := arg(args, 0)

	want, err := strconv.Atoi(spoken)
	if err != nil {
		return fmt.Errorf("expected an HTTP status code, got %q: %w", spoken, err)
	}

	if s.StatusCode == 0 {
		return fmt.Errorf("expected HTTP %d, but no response was recorded: no When step sent a request", want)
	}

	if s.StatusCode != want {
		return fmt.Errorf("expected HTTP %d, got %d; body: %s", want, s.StatusCode, excerpt(s.Body))
	}

	return nil
}

// resolvePort: the port the step names, else the configured one, else
// the default.
func resolvePort(spoken string) (int, error) {
	if spoken == "" {
		spoken = os.Getenv("MCP_PORT")
	}

	if spoken == "" {
		return defaultPort, nil
	}

	port, err := strconv.Atoi(spoken)
	if err != nil {
		return 0, fmt.Errorf("expected a port number, got %q: %w", spoken, err)
	}

	return port, nil
}

func paramsFor(method, actor string) map[string]any {
	if method != "initialize" {
		return map[string]any{}
	}

	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": actor, "version": "1.0.0"},
	}
}

func arg(args []string, i int) string {
	if i >= len(args) {
		return ""
	}

	return args[i]
}

func excerpt(body []byte) string {
	if len(body) > bodyExcerpt {
		return string(body[:bodyExcerpt]) + "..."
	}

	return string(body)
}
