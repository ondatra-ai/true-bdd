//go:build bdd

// Package steps holds the step definitions for the `mcp` suite
// (tests/mcp), the suite that owns the mcp-service scenarios of
// docs/scenarios.yaml.
//
// Every definition is registered in Register and implemented against
// *State, the single state type this suite shares: Given prepares, When
// acts, Then asserts and returns an error naming what it expected and
// what it got. New definitions belong in Register below — not in a
// second registry.
package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort    = 8080
	readyTimeout   = 10 * time.Second
	readyInterval  = 100 * time.Millisecond
	requestTimeout = 10 * time.Second
	maxBodyBytes   = 1 << 20
)

// State is the per-scenario state every step definition in this suite
// reads and writes. One value exists per scenario run.
type State struct {
	// BaseURL is the root this suite posts to, e.g. http://127.0.0.1:8080.
	// Empty until a Given step has located the server.
	BaseURL string
	// Port is the port the server was found accepting connections on.
	Port int
	// Client is reused by every request-issuing step.
	Client *http.Client

	// LastMethod is the JSON-RPC method of the most recent request.
	LastMethod string
	// LastEndpoint is the full URL of the most recent request.
	LastEndpoint string
	// StatusCode is the HTTP status of the most recent response, or 0
	// when no request has been issued yet.
	StatusCode int
	// Body is the body of the most recent response.
	Body []byte
}

// NewState returns the state a single scenario run starts from.
func NewState() *State {
	return &State{Client: &http.Client{Timeout: requestTimeout}}
}

// StepFunc is the signature of every step definition in this suite: the
// scenario's state, plus the capture groups of the regexp that matched,
// in order.
type StepFunc func(st *State, args ...string) error

type definition struct {
	pattern *regexp.Regexp
	fn      StepFunc
}

// Suite is this package's registry of step definitions.
type Suite struct {
	defs []definition
}

// Step registers one definition. The pattern must be anchored with ^…$.
func (s *Suite) Step(pattern string, fn StepFunc) {
	s.defs = append(s.defs, definition{pattern: regexp.MustCompile(pattern), fn: fn})
}

// NewSuite returns a Suite with every definition of this package
// registered.
func NewSuite() *Suite {
	s := &Suite{}
	Register(s)
	return s
}

// Run executes the one definition whose regexp matches text — the step's
// text with the Given/When/Then/And keyword already stripped. Matching
// none, or more than one, is the step being unbound or ambiguous, and
// either is reported as such rather than guessed at.
func (s *Suite) Run(st *State, text string) error {
	var (
		args     []string
		chosen   StepFunc
		patterns []string
	)
	for _, d := range s.defs {
		m := d.pattern.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		patterns = append(patterns, d.pattern.String())
		args = m[1:]
		chosen = d.fn
	}
	switch len(patterns) {
	case 0:
		return fmt.Errorf("expected step %q to match one definition in tests/mcp/steps, got none", text)
	case 1:
		return chosen(st, args...)
	default:
		return fmt.Errorf("expected step %q to match exactly one definition in tests/mcp/steps, got %d (%s)",
			text, len(patterns), strings.Join(patterns, ", "))
	}
}

// Register registers every step definition of the `mcp` suite. One call
// per step wording the registry uses; capture groups stand where the
// wording varies, so one definition serves every scenario phrasing it
// that way.
func Register(suite *Suite) {
	// Given: "the MCP server is running on its configured port", and the
	// "… on port 8080" phrasing of the same preparation.
	suite.Step(`^the MCP server is running on (?:its configured port|port (\d+))$`, givenMCPServerRunning)

	// When: "the Claude User posts a valid JSON-RPC initialize request to /mcp".
	// Actor, JSON-RPC method and endpoint path all vary between scenarios.
	suite.Step(`^the (.+) posts a valid JSON-RPC ([A-Za-z][A-Za-z0-9_/.-]*) request to (/\S*)$`, whenPostsJSONRPCRequest)

	// Then: "the server returns HTTP 200", and every other status a
	// scenario asserts.
	suite.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// givenMCPServerRunning resolves where the MCP server listens and waits
// until it accepts connections there. The port comes from the step text
// when the scenario names one, else MCP_PORT, else the default; a full
// MCP_BASE_URL overrides the lot so the same scenarios can run against
// a non-local deployment.
func givenMCPServerRunning(st *State, args ...string) error {
	if st.Client == nil {
		st.Client = &http.Client{Timeout: requestTimeout}
	}

	port := defaultPort
	source := fmt.Sprintf("the suite default (%d)", defaultPort)
	if raw := arg(args, 0); raw != "" {
		n, err := parsePort(raw)
		if err != nil {
			return fmt.Errorf("expected the port named in the step text to be a number in 1..65535, got %q", raw)
		}
		port, source = n, "the step text"
	} else if raw := strings.TrimSpace(os.Getenv("MCP_PORT")); raw != "" {
		n, err := parsePort(raw)
		if err != nil {
			return fmt.Errorf("expected MCP_PORT to be a number in 1..65535, got %q", raw)
		}
		port, source = n, "MCP_PORT"
	}

	base := strings.TrimRight(strings.TrimSpace(os.Getenv("MCP_BASE_URL")), "/")
	if base == "" {
		base = fmt.Sprintf("http://127.0.0.1:%d", port)
	} else {
		source = "MCP_BASE_URL"
	}

	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("expected a base URL like http://127.0.0.1:%d (from %s), got %q", defaultPort, source, base)
	}

	dialPort := u.Port()
	if dialPort == "" {
		if u.Scheme == "https" {
			dialPort = "443"
		} else {
			dialPort = "80"
		}
	}
	if n, convErr := strconv.Atoi(dialPort); convErr == nil {
		port = n
	}

	st.BaseURL = base
	st.Port = port

	addr := net.JoinHostPort(u.Hostname(), dialPort)
	deadline := time.Now().Add(readyTimeout)
	var lastErr error
	for {
		conn, dialErr := net.DialTimeout("tcp", addr, readyInterval*5)
		if dialErr == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = dialErr
		if time.Now().After(deadline) {
			return fmt.Errorf("expected the MCP server to be accepting connections on %s (resolved from %s) within %s, got %v",
				addr, source, readyTimeout, lastErr)
		}
		time.Sleep(readyInterval)
	}
}

// whenPostsJSONRPCRequest posts a well-formed JSON-RPC 2.0 request for
// the captured method to the captured path, as the captured actor, and
// records the status and body for the Then steps to assert on.
func whenPostsJSONRPCRequest(st *State, args ...string) error {
	actor := arg(args, 0)
	method := arg(args, 1)
	path := arg(args, 2)

	if st.BaseURL == "" {
		return fmt.Errorf("expected an earlier Given step to have started the MCP server before %q posts a JSON-RPC %q request to %s, got no base URL in the suite state",
			actor, method, path)
	}
	if st.Client == nil {
		st.Client = &http.Client{Timeout: requestTimeout}
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	}
	if params := paramsFor(method, actor); params != nil {
		payload["params"] = params
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("expected to marshal a valid JSON-RPC %q request for %q, got %v", method, actor, err)
	}

	endpoint := st.BaseURL + path
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("expected to build a POST to %s, got %v", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := st.Client.Do(req)
	if err != nil {
		return fmt.Errorf("expected POST %s (JSON-RPC %q as %q) to produce a response, got %v", endpoint, method, actor, err)
	}
	defer func() { _ = resp.Body.Close() }()

	read, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("expected to read the response body of POST %s, got %v", endpoint, err)
	}

	st.LastMethod = method
	st.LastEndpoint = endpoint
	st.StatusCode = resp.StatusCode
	st.Body = read
	return nil
}

// thenServerReturnsStatus asserts the status of the most recent
// response. The error names the expected status, the one received, and
// a slice of the body, because that error is the failure a reader gets.
func thenServerReturnsStatus(st *State, args ...string) error {
	raw := arg(args, 0)
	want, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("expected the status named in the step text to be a three-digit number, got %q", raw)
	}
	if st.StatusCode == 0 {
		return fmt.Errorf("expected HTTP %d, got no response at all: no earlier When step issued a request", want)
	}
	if st.StatusCode != want {
		return fmt.Errorf("expected HTTP %d from POST %s (JSON-RPC %q), got %d; body: %s",
			want, st.LastEndpoint, st.LastMethod, st.StatusCode, snippet(st.Body, 300))
	}
	return nil
}

// paramsFor returns the params a valid request of the named JSON-RPC
// method carries. actor names the client in the handshake, so a
// scenario reading "the Claude User posts …" identifies itself as that
// actor on the wire.
func paramsFor(method, actor string) map[string]any {
	switch method {
	case "initialize":
		name := strings.TrimSpace(actor)
		if name == "" {
			name = "bdd-suite"
		}
		return map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    name,
				"version": "1.0.0",
			},
		}
	case "tools/list", "resources/list", "prompts/list":
		return map[string]any{}
	default:
		return nil
	}
}

// arg returns capture group i, trimmed, or "" when the group did not
// participate in the match.
func arg(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return strings.TrimSpace(args[i])
}

func parsePort(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n <= 0 || n > 65535 {
		return 0, fmt.Errorf("port %d out of range", n)
	}
	return n, nil
}

func snippet(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "<empty>"
	}
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
