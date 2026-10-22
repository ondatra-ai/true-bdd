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

// defaultTimeout bounds every step that talks to the MCP server, so an
// unresponsive server fails the scenario instead of hanging it.
const defaultTimeout = 10 * time.Second

// Register binds the registry's step vocabulary to executable code.
//
// Every pattern is anchored and carries a capture group wherever the
// step text varies — the port, the JSON-RPC method, the request path,
// the status code — so one definition serves every scenario that
// phrases the same action with different values.
func Register(s Suite) {
	s.Step(`^the MCP server is running on (?:its configured port|port "?(\d+)"?)$`, givenMCPServerRunning)
	s.Step(`^the Claude User posts a valid JSON-RPC "?([A-Za-z][A-Za-z0-9_/.-]*)"? request to (\S+)$`, whenClaudeUserPostsJSONRPC)
	s.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// givenMCPServerRunning prepares: it asserts the suite handed us a
// reachable MCP base URL, on the port the scenario names (or on
// MCP_PORT when the scenario says "its configured port"), and makes
// sure a client is available for the When step.
func givenMCPServerRunning(st *State, args []string) error {
	if st == nil {
		return fmt.Errorf("expected scenario state to be initialised before the Given step; got nil *State")
	}
	if strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("expected the suite to have started an MCP server and recorded its address in State.BaseURL; got an empty BaseURL")
	}
	if st.Client == nil {
		st.Client = &http.Client{Timeout: defaultTimeout}
	}

	base, err := url.Parse(st.BaseURL)
	if err != nil {
		return fmt.Errorf("expected State.BaseURL to be a valid URL; got %q (%v)", st.BaseURL, err)
	}
	if base.Host == "" {
		return fmt.Errorf("expected State.BaseURL to carry a host:port; got %q", st.BaseURL)
	}

	// The scenario either names a port explicitly or defers to the
	// configured one, which the suite exports as MCP_PORT.
	wantPort := ""
	if n, ok := firstInt(args); ok {
		wantPort = strconv.Itoa(n)
	} else {
		wantPort = strings.TrimSpace(os.Getenv("MCP_PORT"))
	}
	if wantPort != "" && base.Port() != wantPort {
		return fmt.Errorf("expected the MCP server to be listening on port %s, but %s reports port %q", wantPort, st.BaseURL, base.Port())
	}

	conn, err := net.DialTimeout("tcp", base.Host, defaultTimeout)
	if err != nil {
		return fmt.Errorf("expected the MCP server to be accepting connections on %s; dialling it failed: %v", base.Host, err)
	}
	_ = conn.Close()

	// A fresh scenario must not inherit a previous response.
	if st.Response != nil && st.Response.Body != nil {
		_ = st.Response.Body.Close()
	}
	st.Response = nil
	return nil
}

// whenClaudeUserPostsJSONRPC acts: it POSTs a well-formed JSON-RPC 2.0
// request for the captured method to the captured path and records the
// response on the scenario state for the Then steps to assert against.
func whenClaudeUserPostsJSONRPC(st *State, args []string) error {
	if st == nil {
		return fmt.Errorf("expected scenario state to be initialised before the When step; got nil *State")
	}
	if strings.TrimSpace(st.BaseURL) == "" {
		return fmt.Errorf("expected State.BaseURL to be set by the Given step before posting; got an empty BaseURL")
	}
	if st.Client == nil {
		st.Client = &http.Client{Timeout: defaultTimeout}
	}

	captured := tail(args, 2)
	if len(captured) != 2 || captured[0] == "" || captured[1] == "" {
		return fmt.Errorf("expected the step to capture a JSON-RPC method and a request path; got captures %q", args)
	}
	method, path := captured[0], captured[1]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	endpoint := strings.TrimSuffix(st.BaseURL, "/") + path

	payload := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":%q,"params":{"protocolVersion":"2026-10-22","capabilities":{},"clientInfo":{"name":"true-bdd","version":"0.0.0"}}}`,
		method,
	)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("expected to build a POST request for %s; building it failed: %v", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := st.Client.Do(req)
	if err != nil {
		return fmt.Errorf("expected POST %s to reach the MCP server with a JSON-RPC %q request; the request failed: %v", endpoint, method, err)
	}

	// Buffer the body so the connection is released now and the Then
	// steps can still read it.
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("expected to read the response to POST %s; reading its body failed: %v", endpoint, readErr)
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))

	if st.Response != nil && st.Response.Body != nil {
		_ = st.Response.Body.Close()
	}
	st.Response = resp
	return nil
}

// thenServerReturnsStatus asserts: the recorded response carries the
// status code the scenario names, and names both sides plus a body
// excerpt when it does not.
func thenServerReturnsStatus(st *State, args []string) error {
	want, ok := firstInt(args)
	if !ok {
		return fmt.Errorf("expected the step to capture an HTTP status code; got captures %q", args)
	}
	if st == nil || st.Response == nil {
		return fmt.Errorf("expected HTTP %d, but no response was recorded: no When step issued a request against the MCP server", want)
	}
	if st.Response.StatusCode != want {
		if snippet := bodySnippet(st.Response); snippet != "" {
			return fmt.Errorf("expected HTTP %d from %s, got HTTP %d; body: %s", want, requestTarget(st.Response), st.Response.StatusCode, snippet)
		}
		return fmt.Errorf("expected HTTP %d from %s, got HTTP %d", want, requestTarget(st.Response), st.Response.StatusCode)
	}
	return nil
}

// tail returns the last n captures, so a definition reads its groups
// correctly whether or not the harness prefixes the whole step match.
func tail(args []string, n int) []string {
	if n <= 0 || len(args) < n {
		return nil
	}
	out := make([]string, 0, n)
	for _, a := range args[len(args)-n:] {
		out = append(out, strings.TrimSpace(a))
	}
	return out
}

// firstInt returns the first capture that parses as an integer. A
// capture group inside an alternative may not participate in the match,
// so absence is reported rather than guessed at.
func firstInt(args []string) (int, bool) {
	for _, a := range args {
		if n, err := strconv.Atoi(strings.TrimSpace(a)); err == nil {
			return n, true
		}
	}
	return 0, false
}

// requestTarget describes the request a response came from, for errors.
func requestTarget(resp *http.Response) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.Method + " " + resp.Request.URL.String()
	}
	return "the last request"
}

// bodySnippet reads a short, rewindable excerpt of the response body so
// a failure message can show what the server actually said.
func bodySnippet(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return ""
	}
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), resp.Body))
	return strings.TrimSpace(string(raw))
}
