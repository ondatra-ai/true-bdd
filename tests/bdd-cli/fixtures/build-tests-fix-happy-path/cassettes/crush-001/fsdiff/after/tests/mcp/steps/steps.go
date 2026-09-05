//go:build bdd

// Package steps binds the mcp-service scenarios' step text to executable
// Go. The runner matches a step's text — keyword stripped — against the
// patterns Register installs and calls the single definition that matches.
package steps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort    = "8080"
	dialTimeout    = 2 * time.Second
	requestTimeout = 10 * time.Second
	maxBodySnippet = 200
)

// StepFunc runs one step against the scenario state. args holds the
// pattern's capture groups in order; a group that did not participate is
// the empty string.
type StepFunc func(state *State, args []string) error

// Suite is what the scenario runner offers this package: one call per
// definition, anchored pattern plus body.
type Suite interface {
	Step(pattern string, fn StepFunc)
}

// Response is the last HTTP reply a When step received.
type Response struct {
	Status int
	Body   []byte
}

// State is the per-scenario state every definition in this package shares.
type State struct {
	BaseURL  string
	Response *Response
}

// NewState returns the state a scenario starts from.
func NewState() *State {
	return &State{}
}

// Register installs every step definition this suite binds.
func Register(suite Suite) {
	suite.Step(
		`^the (.+) server is running on (?:its configured port|port (\d+))$`,
		givenServerRunning,
	)
	suite.Step(
		`^the (.+) posts a valid JSON-RPC ([A-Za-z][\w/.-]*) request to (/\S*)$`,
		whenPostsJSONRPC,
	)
	suite.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// givenServerRunning records the base URL the When steps post to, and
// proves the port is actually accepting connections before any of them run.
func givenServerRunning(state *State, args []string) error {
	name := arg(args, 0)

	port := arg(args, 1)
	if port == "" {
		port = defaultPort
	}

	addr := net.JoinHostPort("127.0.0.1", port)

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf(
			"expected the %s server to be listening on %s, got no connection: %w",
			name, addr, err,
		)
	}

	_ = conn.Close()
	state.BaseURL = "http://" + addr

	return nil
}

// whenPostsJSONRPC posts one JSON-RPC request and records the reply for
// the Then steps to assert on.
func whenPostsJSONRPC(state *State, args []string) error {
	role, method, path := arg(args, 0), arg(args, 1), arg(args, 2)
	if state.BaseURL == "" {
		return errors.New(
			"expected a running server from a Given step, got none: no base URL recorded",
		)
	}

	url := state.BaseURL + path

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, strings.NewReader(jsonRPCBody(method, role)),
	)
	if err != nil {
		return fmt.Errorf("expected to build a POST to %s, got: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf(
			"expected a reply to the JSON-RPC %s POST to %s, got: %w", method, url, err,
		)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("expected to read the reply body from %s, got: %w", url, err)
	}

	state.Response = &Response{Status: resp.StatusCode, Body: body}

	return nil
}

// thenServerReturnsStatus asserts the recorded status code, naming both
// sides and the body when they disagree.
func thenServerReturnsStatus(state *State, args []string) error {
	raw := arg(args, 0)

	want, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf(
			"expected a numeric HTTP status in the step text, got %q: %w", raw, err,
		)
	}

	if state.Response == nil {
		return fmt.Errorf(
			"expected HTTP %d, got no response at all: no request step ran before this one",
			want,
		)
	}

	if state.Response.Status != want {
		return fmt.Errorf(
			"expected HTTP %d, got %d (body: %s)",
			want, state.Response.Status, bodySnippet(state.Response.Body),
		)
	}

	return nil
}

// jsonRPCBody renders a well-formed JSON-RPC request; initialize carries
// the handshake params the MCP spec requires of it.
func jsonRPCBody(method, role string) string {
	params := "{}"
	if method == "initialize" {
		params = fmt.Sprintf(
			`{"protocolVersion":"2024-11-05","capabilities":{},`+
				`"clientInfo":{"name":%q,"version":"1.0.0"}}`,
			role,
		)
	}

	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)
}

func bodySnippet(body []byte) string {
	if len(body) > maxBodySnippet {
		return string(body[:maxBodySnippet]) + "..."
	}

	return string(body)
}

func arg(args []string, i int) string {
	if i >= len(args) {
		return ""
	}

	return args[i]
}
