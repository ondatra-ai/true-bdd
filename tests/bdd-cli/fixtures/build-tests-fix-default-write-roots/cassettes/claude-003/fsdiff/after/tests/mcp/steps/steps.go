package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Register binds the registry's step wording to this suite's definitions.
//
// Every pattern is anchored and parameterised: the actor, the JSON-RPC
// method, the endpoint path and the expected status code are capture
// groups, so one definition serves every scenario phrasing a step that
// way instead of only the one that first needed it.
func Register(suite Suite) {
	suite.Step(`^the ([A-Za-z0-9 ._-]+) server is running on its configured port$`, givenServerRunningOnConfiguredPort)
	suite.Step(`^the ([A-Za-z0-9 ._-]+) posts a valid JSON-RPC ([A-Za-z0-9/._-]+) request to (\S+)$`, whenPostsValidJSONRPCRequest)
	suite.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsHTTPStatus)
}

// defaultClientTimeout bounds every request a step makes, so a hung
// server fails the scenario with a message instead of the whole suite
// with a timeout.
const defaultClientTimeout = 10 * time.Second

// group returns capture i (0-based) of a pattern carrying n groups.
//
// Harnesses differ on whether the slice handed to a step definition
// starts with the full match or with the first submatch, so accept both
// shapes rather than silently reading the wrong field.
func group(args []string, i, n int) (string, error) {
	switch len(args) {
	case n:
		// submatches only
	case n + 1:
		args = args[1:] // leading full match
	default:
		return "", fmt.Errorf("expected %d capture(s) from the step pattern, got %d: %q", n, len(args), args)
	}
	return args[i], nil
}

// client returns the scenario's HTTP client, creating a bounded one if
// the harness did not supply it.
func client(st *State) *http.Client {
	if st.Client == nil {
		st.Client = &http.Client{Timeout: defaultClientTimeout}
	}
	return st.Client
}

// baseURL validates the scenario's server address and reports what was
// missing when it is not usable.
func baseURL(st *State, what string) (*url.URL, error) {
	if strings.TrimSpace(st.BaseURL) == "" {
		return nil, fmt.Errorf("expected a running server base URL before %s, got an empty State.BaseURL", what)
	}
	address, err := url.Parse(st.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("expected State.BaseURL to be a URL before %s, got %q: %w", what, st.BaseURL, err)
	}
	if address.Host == "" {
		return nil, fmt.Errorf("expected State.BaseURL to carry a host before %s, got %q", what, st.BaseURL)
	}
	return address, nil
}

// givenServerRunningOnConfiguredPort prepares: it proves the named
// server is listening, and that it is listening on the port the
// configuration names.
func givenServerRunningOnConfiguredPort(st *State, args []string) error {
	name, err := group(args, 0, 1)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("expected scenario state for the %s server step, got nil", name)
	}
	address, err := baseURL(st, fmt.Sprintf("checking the %s server is running", name))
	if err != nil {
		return err
	}
	port := address.Port()
	if port == "" {
		return fmt.Errorf("expected the %s server base URL to carry a port, got %q", name, st.BaseURL)
	}
	if configured := strings.TrimSpace(os.Getenv("MCP_PORT")); configured != "" && configured != port {
		return fmt.Errorf("expected the %s server to listen on its configured port %s (MCP_PORT), got port %s from %q", name, configured, port, st.BaseURL)
	}
	conn, err := net.DialTimeout("tcp", address.Host, 2*time.Second)
	if err != nil {
		return fmt.Errorf("expected the %s server to be accepting connections on %s, got dial error: %w", name, address.Host, err)
	}
	_ = conn.Close()
	client(st)
	if st.Response != nil && st.Response.Body != nil {
		_ = st.Response.Body.Close()
	}
	st.Response = nil
	return nil
}

// whenPostsValidJSONRPCRequest acts: it POSTs a well-formed JSON-RPC 2.0
// envelope for the captured method to the captured path and records the
// response for the Then steps.
func whenPostsValidJSONRPCRequest(st *State, args []string) error {
	actor, err := group(args, 0, 3)
	if err != nil {
		return err
	}
	method, err := group(args, 1, 3)
	if err != nil {
		return err
	}
	path, err := group(args, 2, 3)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("expected scenario state before %s posts a JSON-RPC %s request, got nil", actor, method)
	}
	if _, err := baseURL(st, fmt.Sprintf("%s posts a JSON-RPC %s request to %s", actor, method, path)); err != nil {
		return err
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params": map[string]any{
			"protocolVersion": "2026-10-22",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    actor,
				"version": "1.0.0",
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("expected to encode a valid JSON-RPC %s request body, got: %w", method, err)
	}
	endpoint, err := url.JoinPath(st.BaseURL, path)
	if err != nil {
		return fmt.Errorf("expected to build a request URL from base %q and path %q, got: %w", st.BaseURL, path, err)
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("expected to build a POST request to %s, got: %w", endpoint, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if st.Response != nil && st.Response.Body != nil {
		_ = st.Response.Body.Close()
		st.Response = nil
	}
	response, err := client(st).Do(request)
	if err != nil {
		return fmt.Errorf("expected POST %s with a JSON-RPC %s body to reach the server, got: %w", endpoint, method, err)
	}
	st.Response = response
	return nil
}

// thenServerReturnsHTTPStatus asserts the recorded response's status
// code, naming both the expected and the observed status.
func thenServerReturnsHTTPStatus(st *State, args []string) error {
	raw, err := group(args, 0, 1)
	if err != nil {
		return err
	}
	want, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("expected the step to name a numeric HTTP status, got %q", raw)
	}
	if st == nil {
		return fmt.Errorf("expected scenario state when asserting HTTP %d, got nil", want)
	}
	if st.Response == nil {
		return fmt.Errorf("expected an HTTP %d response, got none: no request was recorded in State.Response by any earlier step", want)
	}
	if st.Response.StatusCode != want {
		return fmt.Errorf("expected HTTP %d %s, got HTTP %d %s", want, http.StatusText(want), st.Response.StatusCode, http.StatusText(st.Response.StatusCode))
	}
	return nil
}
