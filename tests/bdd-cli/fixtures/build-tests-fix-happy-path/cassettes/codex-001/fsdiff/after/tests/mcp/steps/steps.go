package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Register binds this suite's step vocabulary to the harness.
//
// One definition per phrasing in docs/scenarios.yaml. Every pattern is
// anchored with ^…$ so it cannot quietly swallow a step it was not
// written for, and carries a capture group wherever a scenario's wording
// varies — the component name, the acting role, the JSON-RPC method, the
// request path, the expected status code — so one definition serves every
// scenario that phrases its step that way rather than only the first.
func Register(s Suite) {
	s.Step(`^the ([A-Za-z0-9 ._-]+) server is running on its configured port$`, givenServerRunningOnConfiguredPort)
	s.Step(`^the ([A-Za-z0-9 ._-]+) posts a valid JSON-RPC ([A-Za-z0-9/._-]+) request to (/\S*)$`, whenPostsValidJSONRPCRequest)
	s.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsHTTPStatus)
}

// stepClientTimeout bounds a step's HTTP call so a hung server fails the
// scenario with a message instead of stalling the suite.
const stepClientTimeout = 10 * time.Second

// Given: the suite's server is up and reachable on the port the scenario
// configured, and the state carries a client ready to talk to it.
func givenServerRunningOnConfiguredPort(state *State, args []string) error {
	captured, err := captures(args, 1)
	if err != nil {
		return fmt.Errorf("given the server is running on its configured port: %w", err)
	}
	component := strings.TrimSpace(captured[0])

	if state == nil {
		return fmt.Errorf("given the %s server is running on its configured port: expected initialised scenario state, got nil", component)
	}
	base := strings.TrimSpace(state.BaseURL)
	if base == "" {
		return fmt.Errorf("given the %s server is running on its configured port: expected State.BaseURL to carry the server address, got an empty string", component)
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("given the %s server is running on its configured port: expected State.BaseURL to be a URL, got %q: %w", component, base, err)
	}
	if parsed.Host == "" {
		return fmt.Errorf("given the %s server is running on its configured port: expected State.BaseURL to carry a host:port, got %q", component, base)
	}
	if configured := strings.TrimSpace(os.Getenv("MCP_PORT")); configured != "" && parsed.Port() != configured {
		return fmt.Errorf(
			"given the %s server is running on its configured port: expected the server to listen on the configured port %s, got port %q from base URL %q",
			component, configured, parsed.Port(), base,
		)
	}

	state.BaseURL = strings.TrimSuffix(base, "/")
	if state.Client == nil {
		state.Client = &http.Client{Timeout: stepClientTimeout}
	}
	state.Response = nil

	// Proof of life: any HTTP reply means the server is running. Only a
	// transport failure means it is not.
	probe, err := state.Client.Get(state.BaseURL + "/")
	if err != nil {
		return fmt.Errorf(
			"given the %s server is running on its configured port: expected %s to accept connections, got transport error: %w",
			component, state.BaseURL, err,
		)
	}
	_, _ = io.Copy(io.Discard, probe.Body)
	_ = probe.Body.Close()
	return nil
}

// When: the named role POSTs a well-formed JSON-RPC request for the named
// method to the given path, and the reply is recorded for the Then steps.
func whenPostsValidJSONRPCRequest(state *State, args []string) error {
	captured, err := captures(args, 3)
	if err != nil {
		return fmt.Errorf("when a valid JSON-RPC request is posted: %w", err)
	}
	role := strings.TrimSpace(captured[0])
	method := strings.TrimSpace(captured[1])
	path := strings.TrimSpace(captured[2])

	if state == nil {
		return fmt.Errorf("when %s posts a valid JSON-RPC %s request to %s: expected initialised scenario state, got nil", role, method, path)
	}
	if strings.TrimSpace(state.BaseURL) == "" {
		return fmt.Errorf(
			"when %s posts a valid JSON-RPC %s request to %s: expected a running server with a base URL, got an empty State.BaseURL",
			role, method, path,
		)
	}
	if state.Client == nil {
		state.Client = &http.Client{Timeout: stepClientTimeout}
	}

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  jsonRPCParams(method),
	})
	if err != nil {
		return fmt.Errorf(
			"when %s posts a valid JSON-RPC %s request to %s: expected to encode the request body, got: %w",
			role, method, path, err,
		)
	}

	target := strings.TrimSuffix(state.BaseURL, "/") + path
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf(
			"when %s posts a valid JSON-RPC %s request to %s: expected to build a POST for %q, got: %w",
			role, method, path, target, err,
		)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := state.Client.Do(request)
	if err != nil {
		return fmt.Errorf(
			"when %s posts a valid JSON-RPC %s request to %s: expected an HTTP response from %s, got transport error: %w",
			role, method, path, target, err,
		)
	}

	// Drain and re-seat the body so the connection is released now and
	// later assertions can still read what the server said.
	raw, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return fmt.Errorf(
			"when %s posts a valid JSON-RPC %s request to %s: expected to read the HTTP %d response body, got: %w",
			role, method, path, response.StatusCode, readErr,
		)
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	state.Response = response
	return nil
}

// Then: the recorded response carries the expected HTTP status code.
func thenServerReturnsHTTPStatus(state *State, args []string) error {
	captured, err := captures(args, 1)
	if err != nil {
		return fmt.Errorf("then the server returns an HTTP status: %w", err)
	}
	raw := strings.TrimSpace(captured[0])
	want, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("then the server returns HTTP %s: expected a numeric status code in the step text, got %q: %w", raw, raw, err)
	}

	if state == nil || state.Response == nil {
		return fmt.Errorf(
			"then the server returns HTTP %d: expected a response recorded by a preceding When step, got none",
			want,
		)
	}
	if state.Response.StatusCode != want {
		return fmt.Errorf(
			"then the server returns HTTP %d (%s): got HTTP %d (%s) from %s; body: %s",
			want, http.StatusText(want),
			state.Response.StatusCode, http.StatusText(state.Response.StatusCode),
			responseTarget(state.Response), bodySnippet(state.Response),
		)
	}
	return nil
}

// captures normalises the harness's argument slice. A harness may pass the
// full match ahead of the capture groups or the groups alone; a definition
// knows how many groups its own pattern declares, and the groups are the
// trailing n entries either way.
func captures(args []string, n int) ([]string, error) {
	if len(args) < n {
		return nil, fmt.Errorf("expected %d captured argument(s) from the step pattern, got %d: %q", n, len(args), args)
	}
	return args[len(args)-n:], nil
}

// jsonRPCParams supplies the params that make a request for the given
// method *valid*, which is what the step wording promises.
func jsonRPCParams(method string) map[string]any {
	if strings.EqualFold(method, "initialize") {
		return map[string]any{
			"protocolVersion": "2026-10-22",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "true-bdd-mcp-suite",
				"version": "1.0.0",
			},
		}
	}
	return map[string]any{}
}

// responseTarget names the URL a response came from, for failure messages.
func responseTarget(response *http.Response) string {
	if response.Request != nil && response.Request.URL != nil {
		return response.Request.URL.String()
	}
	return "the server"
}

// bodySnippet renders the response body for a failure message and re-seats
// it so a later assertion can read it too.
func bodySnippet(response *http.Response) string {
	if response.Body == nil {
		return "<empty>"
	}
	raw, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return fmt.Sprintf("<unreadable: %v>", err)
	}
	if len(raw) == 0 {
		return "<empty>"
	}
	const limit = 512
	if len(raw) > limit {
		return string(raw[:limit]) + "…(truncated)"
	}
	return string(raw)
}
