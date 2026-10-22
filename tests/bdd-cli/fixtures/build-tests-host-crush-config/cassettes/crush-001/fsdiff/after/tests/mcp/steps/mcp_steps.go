//go:build bdd

// Step definitions for the `mcp` suite.
//
// The suite that owns scenario INT-900 is rooted at tests/mcp per
// docs/architecture/architecture.yaml, so every definition for its
// scenarios is registered from this package.
//
// Shape of the package:
//
//   - State is the suite's shared state: one HTTP client, the resolved
//     base URL of the server under test, and whatever the last request
//     came back with. Every definition reads and writes it through the
//     closure Register hands out, so Given can prepare for a When and a
//     Then can report on what actually happened.
//   - Patterns are anchored and parameterised. Where a scenario's wording
//     varies — the actor, the service name, the JSON-RPC method, the
//     request path, the expected status code — there is a capture group,
//     so one definition serves every scenario phrased that way.
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
	"strconv"
	"strings"
	"time"
)

// Suite is the registration surface the BDD harness hands to Register.
// It is an interface so the harness's own suite type satisfies it
// structurally, without this package having to import the harness.
type Suite interface {
	Step(pattern string, fn interface{})
}

// State is the state shared by every definition in this suite.
type State struct {
	Client  *http.Client
	Service string // service named by the Given step, for error messages
	BaseURL string // e.g. "http://127.0.0.1:8080", no trailing slash

	LastMethod string // JSON-RPC method of the last request sent
	LastPath   string // request path of the last request sent
	LastStatus int    // 0 until a response has been received
	LastBody   []byte
}

const (
	defaultHost    = "127.0.0.1"
	defaultPort    = "8080"
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
)

// resolveBaseURL derives the address of the server under test from the
// environment: MCP_BASE_URL wins, otherwise MCP_PORT on localhost,
// otherwise the default port. "its configured port" in the scenario text
// is exactly this configuration.
func resolveBaseURL() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("MCP_BASE_URL")); raw != "" {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("expected MCP_BASE_URL to be a URL like http://127.0.0.1:8080, got %q: %w", raw, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("expected MCP_BASE_URL to be a URL like http://127.0.0.1:8080, got %q", raw)
		}
		return strings.TrimRight(raw, "/"), nil
	}

	port := strings.TrimSpace(os.Getenv("MCP_PORT"))
	if port == "" {
		port = defaultPort
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("expected MCP_PORT to be a port number, got %q", port)
	}
	return "http://" + net.JoinHostPort(defaultHost, port), nil
}

// hostPort returns the dialable host:port of a base URL, filling in the
// scheme's default port when the URL omits one.
func hostPort(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("expected a parsable base URL, got %q: %w", base, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("expected a base URL with a host, got %q", base)
	}
	if u.Port() != "" {
		return u.Host, nil
	}
	if u.Scheme == "https" {
		return net.JoinHostPort(u.Hostname(), "443"), nil
	}
	return net.JoinHostPort(u.Hostname(), "80"), nil
}

// waitReachable blocks until something accepts TCP connections at addr, or
// the timeout runs out.
func waitReachable(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		last = err
		if !time.Now().Before(deadline) {
			return fmt.Errorf("expected something listening on %s within %s, but dialling it kept failing: %w", addr, timeout, last)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// jsonRPCParams returns a well-formed params object for the given
// JSON-RPC method, so "a valid ... request" is actually valid.
func jsonRPCParams(method string) map[string]interface{} {
	switch method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "true-bdd-suite",
				"version": "1.0.0",
			},
		}
	default:
		return map[string]interface{}{}
	}
}

// post sends a JSON body to path on the state's base URL and records the
// outcome on the state.
func (s *State) post(path string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("expected the request body to encode as JSON, but encoding failed: %w", err)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	target := s.BaseURL + path

	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("expected to build a POST to %s, but it failed: %w", target, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("expected a response to POST %s, but the request failed: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return fmt.Errorf("POST %s returned HTTP %d but its body could not be read: %w", target, resp.StatusCode, readErr)
	}

	s.LastPath = path
	s.LastStatus = resp.StatusCode
	s.LastBody = raw
	return nil
}

// snippet renders a response body short enough to belong in an error.
func snippet(b []byte) string {
	const max = 300
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "<empty body>"
	}
	if len(t) > max {
		return t[:max] + "…"
	}
	return t
}

// Register binds this suite's vocabulary. Every definition shares the one
// *State created here.
func Register(suite Suite) {
	st := &State{Client: &http.Client{Timeout: requestTimeout}}

	// Given — prepares: resolve the configured address and prove something
	// is listening there before any scenario tries to talk to it.
	// Capture group: the service name ("the MCP server", "the gateway
	// server", …).
	suite.Step(`^the (.+?) server is running on its configured port$`, func(service string) error {
		base, err := resolveBaseURL()
		if err != nil {
			return fmt.Errorf("cannot locate the %s server: %w", service, err)
		}
		addr, err := hostPort(base)
		if err != nil {
			return fmt.Errorf("cannot locate the %s server: %w", service, err)
		}

		st.Service = service
		st.BaseURL = base
		st.LastMethod = ""
		st.LastPath = ""
		st.LastStatus = 0
		st.LastBody = nil

		if err := waitReachable(addr, dialTimeout); err != nil {
			return fmt.Errorf("expected the %s server to be running on its configured port (%s): %w", service, base, err)
		}
		return nil
	})

	// When — acts: POST a well-formed JSON-RPC request.
	// Capture groups: the actor, the JSON-RPC method, the request path.
	suite.Step(`^the (.+?) posts a valid JSON-RPC (\S+) request to (\S+)$`, func(actor, method, path string) error {
		if st.BaseURL == "" {
			return fmt.Errorf("%s cannot post a JSON-RPC %q request to %s: no server address has been resolved — the Given step that starts the server did not run", actor, method, path)
		}
		payload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  method,
			"params":  jsonRPCParams(method),
		}
		st.LastMethod = method
		if err := st.post(path, payload); err != nil {
			return fmt.Errorf("%s posting a valid JSON-RPC %q request to %s: %w", actor, method, path, err)
		}
		return nil
	})

	// Then — asserts: the status code of the last response.
	// Capture group: the expected status code.
	suite.Step(`^the server returns HTTP (\d+)$`, func(want string) error {
		code, err := strconv.Atoi(want)
		if err != nil {
			return fmt.Errorf("expected an HTTP status code in the step text, got %q: %w", want, err)
		}
		if st.LastStatus == 0 {
			return fmt.Errorf("expected HTTP %d, but no request has been sent yet — the When step did not run or did not reach the server", code)
		}
		if st.LastStatus != code {
			return fmt.Errorf("expected HTTP %d from POST %s%s (JSON-RPC %q), got HTTP %d; body: %s",
				code, st.BaseURL, st.LastPath, st.LastMethod, st.LastStatus, snippet(st.LastBody))
		}
		return nil
	})
}
