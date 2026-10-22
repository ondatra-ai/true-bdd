package steps

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// defaultPort is the port the MCP server listens on when MCP_PORT is
	// unset, per the word mapping of "MCP server".
	defaultPort = "7411"

	// readyTimeout bounds the wait for GET /healthz to answer 200.
	readyTimeout = 5 * time.Second

	// pollInterval spaces the /healthz attempts.
	pollInterval = 100 * time.Millisecond

	// initializeBody is the JSON-RPC 2.0 request that opens a session.
	initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
)

// Reply is what a session's opening left behind: the status code and
// headers of the server's answer, which the Then steps read.
type Reply struct {
	Status int
	Header http.Header
	Body   string
}

// Snippet returns the reply's body shortened for an error message.
func (r *Reply) Snippet() string {
	const max = 200
	if len(r.Body) <= max {
		return r.Body
	}
	return r.Body[:max] + "..."
}

// State is shared by every step definition of the mcp suite within one
// scenario: the Given step proves the server is up, the When step opens
// the session, the Then steps read the reply it kept.
type State struct {
	// Client issues every request this suite makes.
	Client *http.Client
	// Ready records that a Given step saw /healthz answer 200.
	Ready bool
	// Actor is the role whose client opened the session, as captured by
	// the When step, so a Then step's error can name it.
	Actor string
	// LastReply is the server's answer to the most recent request; nil
	// until a When step has opened a session.
	LastReply *Reply
}

// NewState returns the state a scenario's definitions share.
func NewState() *State {
	return &State{Client: &http.Client{Timeout: readyTimeout}}
}

// BaseURL resolves the MCP server's address: 127.0.0.1 at the port
// MCP_PORT names, 7411 when it is unset.
func (st *State) BaseURL() string {
	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = defaultPort
	}
	return "http://127.0.0.1:" + port
}

// ActorName names the role whose client opened the session; "client" until
// a When step captured one.
func (st *State) ActorName() string {
	if st.Actor == "" {
		return "client"
	}
	return st.Actor
}

func (st *State) httpClient() *http.Client {
	if st.Client == nil {
		st.Client = &http.Client{Timeout: readyTimeout}
	}
	return st.Client
}

// WaitReady polls GET /healthz at the server's address until it answers
// 200, for at most readyTimeout. A server that never answers fails,
// naming the address polled and the last thing that happened there.
func (st *State) WaitReady() error {
	url := st.BaseURL() + "/healthz"
	deadline := time.Now().Add(readyTimeout)
	last := "no attempt completed"
	for {
		resp, err := st.httpClient().Get(url)
		if err != nil {
			last = "request error: " + err.Error()
		} else {
			status := resp.StatusCode
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if status == http.StatusOK {
				st.Ready = true
				return nil
			}
			last = fmt.Sprintf("answered status %d", status)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf(
				"the MCP server is not running: expected GET %s to answer 200 within %s, but it never did (last attempt: %s)",
				url, readyTimeout, last)
		}
		time.Sleep(pollInterval)
	}
}

// OpenSession POSTs the initialize request to /mcp at the server's address,
// once, and keeps the reply's status code and headers for the steps after.
func (st *State) OpenSession() error {
	url := st.BaseURL() + "/mcp"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(initializeBody)))
	if err != nil {
		return fmt.Errorf("could not build the initialize request for %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := st.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("the %s could not open a session: POST %s (%s) failed: %w",
			st.ActorName(), url, initializeBody, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("the %s could not read the reply to initialize from %s: %w",
			st.ActorName(), url, err)
	}
	st.LastReply = &Reply{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: string(body)}
	return nil
}

// OpeningReply returns the session's opening. A Then step that finds no
// reply fails instead of passing.
func (st *State) OpeningReply() (*Reply, error) {
	if st.LastReply == nil {
		return nil, fmt.Errorf(
			"no reply to read: no session was opened, so the When step that POSTs %s to %s/mcp never ran",
			initializeBody, st.BaseURL())
	}
	return st.LastReply, nil
}
