package steps

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// defaultMCPPort is the port the MCP server listens on when MCP_PORT is unset.
const defaultMCPPort = "7411"

// healthTimeout bounds how long "the MCP server is running" waits for readiness.
const healthTimeout = 5 * time.Second

// healthPollInterval paces the readiness poll.
const healthPollInterval = 50 * time.Millisecond

// initializeRequest is the JSON-RPC 2.0 request that opens a session.
const initializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`

// ticketPattern is the shape of a granted ticket: 32 lowercase hex characters.
var ticketPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// protocolPattern is the shape of a named protocol version: a YYYY-MM-DD date.
var protocolPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Register binds this suite's step vocabulary to its definitions.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running$`, givenMCPServerIsRunning)
	suite.Step(`^the (.+) opens a session$`, whenActorOpensASession)
	suite.Step(`^the session is granted a ticket$`, thenSessionIsGrantedATicket)
	suite.Step(`^the server names its protocol version$`, thenServerNamesItsProtocolVersion)
}

// serverBaseURL resolves the MCP server's address from MCP_PORT, falling back
// to the default port when the variable is unset.
func serverBaseURL() string {
	port := strings.TrimSpace(os.Getenv("MCP_PORT"))
	if port == "" {
		port = defaultMCPPort
	}

	return "http://127.0.0.1:" + port
}

// httpClient returns the Scenario's HTTP client, installing a bounded default
// if the host state carries none.
func httpClient(state *State) *http.Client {
	if state.Client == nil {
		state.Client = &http.Client{Timeout: 10 * time.Second}
	}

	return state.Client
}

// drain reads and closes a reply's body, leaving its status and headers usable.
func drain(response *http.Response) {
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}

// givenMCPServerIsRunning polls GET /healthz at the resolved address until the
// server answers 200, and keeps that address for the steps that follow.
func givenMCPServerIsRunning(state *State, _ []string) error {
	base := serverBaseURL()
	state.BaseURL = base
	health := base + "/healthz"
	deadline := time.Now().Add(healthTimeout)
	last := "no attempt completed"

	for {
		response, err := httpClient(state).Get(health)
		if err == nil {
			status := response.StatusCode
			drain(response)

			if status == http.StatusOK {
				return nil
			}

			last = fmt.Sprintf("status %d", status)
		} else {
			last = err.Error()
		}

		if time.Now().After(deadline) {
			return fmt.Errorf(
				"expected the MCP server to answer GET %s with 200 within %s, got: %s",
				health, healthTimeout, last,
			)
		}

		time.Sleep(healthPollInterval)
	}
}

// whenActorOpensASession POSTs the initialize request to /mcp once, and keeps
// the reply's status and headers for the steps after. The captured role names
// whose client asks; the request the spec maps is the same for every role.
func whenActorOpensASession(state *State, _ []string) error {
	if state.BaseURL == "" {
		state.BaseURL = serverBaseURL()
	}

	endpoint := state.BaseURL + "/mcp"

	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(initializeRequest))
	if err != nil {
		return fmt.Errorf("expected to build the initialize request for POST %s, got: %w", endpoint, err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient(state).Do(request)
	if err != nil {
		return fmt.Errorf("expected POST %s to open a session, got: %w", endpoint, err)
	}

	drain(response)
	state.Response = response

	return nil
}

// thenSessionIsGrantedATicket asserts the session's opening was accepted and
// carried a well-formed ticket.
func thenSessionIsGrantedATicket(state *State, _ []string) error {
	if state.Response == nil {
		return fmt.Errorf("expected a reply to the session's opening, got none: no session was opened")
	}

	ticket := state.Response.Header.Get("X-Session-Ticket")

	if state.Response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"expected the session's opening to be accepted with status 200, got status %d with X-Session-Ticket %q",
			state.Response.StatusCode, ticket,
		)
	}

	if !ticketPattern.MatchString(ticket) {
		return fmt.Errorf(
			"expected X-Session-Ticket to be 32 lowercase hexadecimal characters, got %q on a status %d reply",
			ticket, state.Response.StatusCode,
		)
	}

	return nil
}

// thenServerNamesItsProtocolVersion asserts the session's opening named the
// protocol version the server speaks, as a YYYY-MM-DD date.
func thenServerNamesItsProtocolVersion(state *State, _ []string) error {
	if state.Response == nil {
		return fmt.Errorf("expected a reply to the session's opening, got none: no session was opened")
	}

	version := state.Response.Header.Get("X-MCP-Protocol")

	if !protocolPattern.MatchString(version) {
		return fmt.Errorf(
			"expected X-MCP-Protocol to name a protocol version as a YYYY-MM-DD date, got %q on a status %d reply",
			version, state.Response.StatusCode,
		)
	}

	if _, err := time.Parse("2006-01-02", version); err != nil {
		return fmt.Errorf(
			"expected X-MCP-Protocol to name a protocol version as a valid YYYY-MM-DD date, got %q",
			version,
		)
	}

	return nil
}
