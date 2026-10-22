package steps

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Word mapping (docs/architecture/architecture.yaml):
//   MCP server       - 127.0.0.1 at the port MCP_PORT names, 7411 when unset;
//                      answers GET /healthz with 200 once ready.
//   session          - opened by JSON-RPC 2.0 "initialize" POSTed to /mcp with
//                      Content-Type application/json; the reply is the opening.
//   ticket           - X-Session-Ticket of that reply: 32 lowercase hex chars.
//   protocol version - X-MCP-Protocol of that reply: a date in YYYY-MM-DD form.
const (
	defaultMCPPort    = "7411"
	healthDeadline    = 5 * time.Second
	healthPollEvery   = 50 * time.Millisecond
	initializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
)

var (
	// A ticket is 32 lowercase hexadecimal characters, nothing else.
	ticketPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	// A protocol version is a date in YYYY-MM-DD form, such as 2025-06-18.
	protocolVersionPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// Register binds every phrase this suite's scenarios speak. One definition
// per catalogue phrase, anchored, with a capture group wherever the phrase
// varies, so a new scenario phrasing it the same way needs no new definition.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running$`, givenTheMCPServerIsRunning)
	suite.Step(`^the (.+) opens a session$`, whenTheActorOpensASession)
	suite.Step(`^the session is granted a ticket$`, thenTheSessionIsGrantedATicket)
	suite.Step(`^the server names its protocol version$`, thenTheServerNamesItsProtocolVersion)
}

// serverBaseURL resolves the MCP server's address the way the word mapping
// says: 127.0.0.1 at the port MCP_PORT names, 7411 when it is unset.
func serverBaseURL() string {
	port := strings.TrimSpace(os.Getenv("MCP_PORT"))
	if port == "" {
		port = defaultMCPPort
	}
	return "http://127.0.0.1:" + port
}

// httpClient reuses the client the suite's Init seeded, and falls back to a
// bounded one so a definition never blocks forever.
func httpClient(state *State) *http.Client {
	if state.Client == nil {
		state.Client = &http.Client{Timeout: 10 * time.Second}
	}
	return state.Client
}

// lastReply is the reply to the session's opening, kept by the When step.
// A Then step that finds none fails instead of passing.
func lastReply(state *State) (*http.Response, error) {
	if state.Response == nil {
		return nil, fmt.Errorf(
			"expected a reply to the session's initialize request to be on hand, got none: " +
				"no When step opened a session, or it never reached the MCP server")
	}
	return state.Response, nil
}

// capturedActor reads the role the When step names. The harness hands a
// definition its capture groups; reading the final element keeps this correct
// whether or not the whole matched step text is prepended to them.
func capturedActor(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[len(args)-1])
}

// givenTheMCPServerIsRunning: the server has started and is ready to answer.
// Poll GET /healthz at the resolved address until it answers 200, for at most
// 5 seconds; a server that never answers fails the step, naming the address.
func givenTheMCPServerIsRunning(state *State, _ []string) error {
	state.BaseURL = serverBaseURL()
	health := state.BaseURL + "/healthz"

	var lastOutcome string
	deadline := time.Now().Add(healthDeadline)
	for {
		response, err := httpClient(state).Get(health)
		if err != nil {
			lastOutcome = "request error: " + err.Error()
		} else {
			status := response.StatusCode
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if status == http.StatusOK {
				return nil
			}
			lastOutcome = fmt.Sprintf("status %d", status)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf(
				"expected the MCP server to answer GET %s with 200 within %s, got %s on the last attempt",
				health, healthDeadline, lastOutcome)
		}
		time.Sleep(healthPollEvery)
	}
}

// whenTheActorOpensASession: that role's client asks the MCP server for a new
// session, once, and the server's answer is kept for the steps that follow.
func whenTheActorOpensASession(state *State, args []string) error {
	actor := capturedActor(args)
	if actor == "" {
		return fmt.Errorf("expected the step to name the role opening the session, got an empty role")
	}
	if state.BaseURL == "" {
		state.BaseURL = serverBaseURL()
	}
	endpoint := state.BaseURL + "/mcp"

	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(initializeRequest))
	if err != nil {
		return fmt.Errorf("expected to build the initialize request for POST %s, got: %w", endpoint, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient(state).Do(request)
	if err != nil {
		return fmt.Errorf(
			"expected the %s's client to open a session with POST %s, got: %w", actor, endpoint, err)
	}
	// Only the status code and headers of the opening are observed later;
	// drain and close the body so the connection is released.
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	state.Response = response
	return nil
}

// thenTheSessionIsGrantedATicket: the MCP server accepted the session and
// handed back its ticket - a 200 reply carrying an X-Session-Ticket of 32
// lowercase hexadecimal characters.
func thenTheSessionIsGrantedATicket(state *State, _ []string) error {
	response, err := lastReply(state)
	if err != nil {
		return err
	}
	ticket := response.Header.Get("X-Session-Ticket")
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"expected the session's opening to be accepted with status 200 and an X-Session-Ticket "+
				"of 32 lowercase hexadecimal characters, got status %d with X-Session-Ticket %q",
			response.StatusCode, ticket)
	}
	if ticket == "" {
		return fmt.Errorf(
			"expected the status %d reply to the session's opening to carry an X-Session-Ticket "+
				"of 32 lowercase hexadecimal characters, got no such header",
			response.StatusCode)
	}
	if !ticketPattern.MatchString(ticket) {
		return fmt.Errorf(
			"expected the X-Session-Ticket of the status %d reply to be 32 lowercase hexadecimal "+
				"characters, got %q (%d characters)",
			response.StatusCode, ticket, len(ticket))
	}
	return nil
}

// thenTheServerNamesItsProtocolVersion: the answer to the session's opening
// says which protocol version the server speaks - X-MCP-Protocol, a date in
// YYYY-MM-DD form.
func thenTheServerNamesItsProtocolVersion(state *State, _ []string) error {
	response, err := lastReply(state)
	if err != nil {
		return err
	}
	version := response.Header.Get("X-MCP-Protocol")
	if version == "" {
		return fmt.Errorf(
			"expected the status %d reply to the session's opening to carry an X-MCP-Protocol header "+
				"holding a date in YYYY-MM-DD form, got no such header",
			response.StatusCode)
	}
	if !protocolVersionPattern.MatchString(version) {
		return fmt.Errorf(
			"expected the X-MCP-Protocol of the status %d reply to hold a date in YYYY-MM-DD form, got %q",
			response.StatusCode, version)
	}
	if _, err := time.Parse("2006-01-02", version); err != nil {
		return fmt.Errorf(
			"expected the X-MCP-Protocol of the status %d reply to hold a real date in YYYY-MM-DD form, got %q: %v",
			response.StatusCode, version, err)
	}
	return nil
}
