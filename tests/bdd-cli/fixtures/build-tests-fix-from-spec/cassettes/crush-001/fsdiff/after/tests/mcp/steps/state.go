// Package steps binds the registry steps of the `mcp` suite to code.
//
// The suite's state is the MCP server address the Given step resolved
// and the reply to the session's opening the When step kept; every Then
// step reads that reply.
package steps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// portEnv and defaultPort are the architecture's address mapping:
	// 127.0.0.1 at MCP_PORT, 7411 when it is unset.
	portEnv     = "MCP_PORT"
	defaultPort = "7411"
	localHost   = "127.0.0.1"
)

const (
	healthTimeout      = 5 * time.Second
	healthPollInterval = 50 * time.Millisecond
	healthReqTimeout   = time.Second
	openSessionTimeout = 10 * time.Second
	initializeRequest  = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
)

// Reply is what a Then step may read of the session's opening: the
// status code and the headers of the server's reply to initialize.
type Reply struct {
	Status int
	Header http.Header
}

// State is the state the suite's step definitions share.
type State struct {
	addr  string
	reply *Reply
}

// New returns the state one scenario's steps run against.
func New() *State {
	return &State{}
}

// Addr is the MCP server address the Given step resolved.
func (s *State) Addr() string {
	return s.addr
}

// openedSession returns the reply the When step kept. A Then step that
// finds none fails instead of passing.
func (s *State) openedSession() (*Reply, error) {
	if s.reply == nil {
		return nil, fmt.Errorf(
			"no session was opened: expected the When step to POST initialize to %s/mcp and keep the reply, but no reply was kept",
			s.serverAddrForMessage(),
		)
	}

	return s.reply, nil
}

func (s *State) serverAddrForMessage() string {
	if s.addr == "" {
		return "(no server address resolved)"
	}

	return s.addr
}

// serverAddr resolves the MCP server's address from MCP_PORT, 7411 when
// it is unset.
func serverAddr() string {
	port := os.Getenv(portEnv)
	if port == "" {
		port = defaultPort
	}

	return "http://" + localHost + ":" + port
}

// waitHealthy polls GET /healthz until it answers 200, for at most
// healthTimeout, and names the address it polled when it never does.
func waitHealthy(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()

	client := &http.Client{Timeout: healthReqTimeout}
	last := "no attempt completed"

	for ctx.Err() == nil {
		status, err := getHealth(ctx, client, addr)
		if err == nil && status == http.StatusOK {
			return nil
		}

		if err != nil {
			last = err.Error()
		} else {
			last = fmt.Sprintf("status %d", status)
		}

		time.Sleep(healthPollInterval)
	}

	return fmt.Errorf(
		"the MCP server is not running: expected GET %s/healthz to answer 200 within %s, last attempt gave %s",
		addr, healthTimeout, last,
	)
}

func getHealth(ctx context.Context, client *http.Client, addr string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr+"/healthz", nil)
	if err != nil {
		return 0, fmt.Errorf("building GET %s/healthz: %w", addr, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("GET %s/healthz failed: %w", addr, err)
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

// openSession POSTs the initialize request to /mcp and keeps the reply's
// status code and headers.
func openSession(addr string) (*Reply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), openSessionTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, addr+"/mcp", strings.NewReader(initializeRequest),
	)
	if err != nil {
		return nil, fmt.Errorf("building POST %s/mcp: %w", addr, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s/mcp with %s failed: %w", addr, initializeRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	return &Reply{Status: resp.StatusCode, Header: resp.Header.Clone()}, nil
}
