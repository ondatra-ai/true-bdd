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
)

// Register binds the registry's step vocabulary to executable code.
//
// Every pattern is anchored and carries a capture group wherever the
// registry's wording varies — the server name, the acting role, the
// JSON-RPC method, the endpoint, the status code — so one definition
// serves every scenario that phrases the step the same way.
func Register(s Suite) {
	s.Step(`^the (.+) server is running on its configured port$`, givenServerRunningOnConfiguredPort)
	s.Step(`^the (.+) posts a valid JSON-RPC ([A-Za-z0-9_./-]+) request to (\S+)$`, whenPostsValidJSONRPCRequest)
	s.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// givenServerRunningOnConfiguredPort prepares: it asserts the harness handed
// this scenario a base URL and client, that the URL's port is the configured
// one, and that the port is actually accepting connections.
func givenServerRunningOnConfiguredPort(state *State, args []string) error {
	name := capture(args, 0, "server")
	if state == nil {
		return fmt.Errorf("the %s server is running on its configured port: expected initialised scenario state, got nil", name)
	}
	if state.BaseURL == "" {
		return fmt.Errorf("the %s server is running on its configured port: expected a non-empty base URL in scenario state, got %q", name, state.BaseURL)
	}
	if state.Client == nil {
		return fmt.Errorf("the %s server is running on its configured port: expected an HTTP client in scenario state, got nil", name)
	}
	address, err := url.Parse(state.BaseURL)
	if err != nil {
		return fmt.Errorf("the %s server is running on its configured port: expected base URL %q to parse, got error: %w", name, state.BaseURL, err)
	}
	configured := os.Getenv("MCP_PORT")
	if configured == "" {
		return fmt.Errorf("the %s server is running on its configured port: expected MCP_PORT to name the configured port, got an empty value", name)
	}
	if address.Port() != configured {
		return fmt.Errorf("the %s server is running on its configured port: expected it to listen on configured port %s, got %q (base URL %q)", name, configured, address.Port(), state.BaseURL)
	}
	probe, err := state.Client.Get(state.BaseURL + "/")
	if err != nil {
		return fmt.Errorf("the %s server is running on its configured port: expected %s to accept connections, got error: %w", name, state.BaseURL, err)
	}
	defer func() { _ = probe.Body.Close() }()
	if _, err := io.Copy(io.Discard, probe.Body); err != nil {
		return fmt.Errorf("the %s server is running on its configured port: expected to drain the probe response from %s, got error: %w", name, state.BaseURL, err)
	}
	// A fresh scenario must not inherit a response from an earlier one.
	state.Response = nil
	return nil
}

// whenPostsValidJSONRPCRequest acts: it POSTs a well-formed JSON-RPC request
// for the captured method to the captured endpoint and records the response.
func whenPostsValidJSONRPCRequest(state *State, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("posting a valid JSON-RPC request: expected 3 captures (actor, method, endpoint), got %d: %q", len(args), args)
	}
	actor, method, path := args[0], args[1], args[2]
	if state == nil || state.BaseURL == "" || state.Client == nil {
		return fmt.Errorf("the %s posts a valid JSON-RPC %s request to %s: expected the running-server step to have prepared a base URL and HTTP client, got %s", actor, method, path, describeState(state))
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  validParamsFor(method),
	})
	if err != nil {
		return fmt.Errorf("the %s posts a valid JSON-RPC %s request to %s: expected the request body to marshal, got error: %w", actor, method, path, err)
	}
	endpoint := strings.TrimSuffix(state.BaseURL, "/") + "/" + strings.TrimPrefix(path, "/")
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("the %s posts a valid JSON-RPC %s request to %s: expected to build a POST for %s, got error: %w", actor, method, path, endpoint, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := state.Client.Do(request)
	if err != nil {
		return fmt.Errorf("the %s posts a valid JSON-RPC %s request to %s: expected POST %s to complete, got error: %w", actor, method, path, endpoint, err)
	}
	raw, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return fmt.Errorf("the %s posts a valid JSON-RPC %s request to %s: expected to read the response body from %s, got error: %w", actor, method, path, endpoint, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("the %s posts a valid JSON-RPC %s request to %s: expected to close the response body from %s, got error: %w", actor, method, path, endpoint, closeErr)
	}
	// Re-attach the drained body so later Then steps can still read it.
	response.Body = io.NopCloser(bytes.NewReader(raw))
	state.Response = response
	return nil
}

// thenServerReturnsStatus asserts the recorded response carried the captured
// HTTP status code.
func thenServerReturnsStatus(state *State, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("the server returns HTTP <code>: expected 1 capture (status code), got %d: %q", len(args), args)
	}
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("the server returns HTTP %s: expected a numeric status code, got error: %w", args[0], err)
	}
	if state == nil || state.Response == nil {
		return fmt.Errorf("the server returns HTTP %d: expected a response recorded by the preceding request step, got none", want)
	}
	if state.Response.StatusCode != want {
		return fmt.Errorf("the server returns HTTP %d: expected status %d, got %d (%q) from %s", want, want, state.Response.StatusCode, state.Response.Status, requestTarget(state.Response))
	}
	return nil
}

// validParamsFor returns params that make the named JSON-RPC method a valid
// call. Unknown methods get an empty object rather than a guess.
func validParamsFor(method string) map[string]any {
	if method == "initialize" {
		return map[string]any{
			"protocolVersion": "2026-10-22",
			"clientInfo":      map[string]any{"name": "true-bdd-suite", "version": "1.0.0"},
			"capabilities":    map[string]any{},
		}
	}
	return map[string]any{}
}

// capture returns the nth capture, or fallback when it is absent or empty, so
// a failure message still reads sensibly.
func capture(args []string, n int, fallback string) string {
	if n < len(args) && strings.TrimSpace(args[n]) != "" {
		return args[n]
	}
	return fallback
}

// describeState renders scenario state for a failure message.
func describeState(state *State) string {
	if state == nil {
		return "nil scenario state"
	}
	return fmt.Sprintf("BaseURL=%q, Client set=%t", state.BaseURL, state.Client != nil)
}

// requestTarget names the URL a response came from, for failure messages.
func requestTarget(response *http.Response) string {
	if response.Request == nil || response.Request.URL == nil {
		return "an unrecorded URL"
	}
	return response.Request.URL.String()
}
