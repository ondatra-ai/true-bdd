package steps

import (
	"fmt"
	"net/http"
	"regexp"
	"time"
)

const (
	ticketHeader   = "X-Session-Ticket"
	protocolHeader = "X-MCP-Protocol"

	// ticketPattern and protocolLayout are the architecture's shapes for
	// the two headers of the session's opening.
	ticketPattern  = `^[0-9a-f]{32}$`
	protocolLayout = "2006-01-02"
)

// Suite is what a step definition is registered with: the runner, which
// matches a step's text and calls the definition with the pattern's
// capture groups.
type Suite interface {
	Step(pattern string, fn func(state *State, args []string) error)
}

// Register binds every step of the `mcp` suite; one call site lists
// everything the suite binds.
func Register(suite Suite) {
	suite.Step(`^the MCP server is running$`, serverIsRunning)
	suite.Step(`^the (.+) opens a session$`, opensSession)
	suite.Step(`^the session is granted a ticket$`, sessionIsGrantedTicket)
	suite.Step(`^the server names its protocol version$`, serverNamesProtocolVersion)
}

// serverIsRunning resolves the server's address and waits until it
// answers GET /healthz with 200.
func serverIsRunning(state *State, _ []string) error {
	addr := serverAddr()
	if err := waitHealthy(addr); err != nil {
		return err
	}

	state.addr = addr
	state.reply = nil

	return nil
}

// opensSession asks the MCP server for a new session once, and keeps the
// reply for the steps that follow.
func opensSession(state *State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("expected the step pattern to capture the role, got %d capture group(s)", len(args))
	}

	role := args[0]

	if state.addr == "" {
		return fmt.Errorf(
			"the %s cannot open a session: no MCP server address was resolved, so the Given step %q must run first",
			role, "the MCP server is running",
		)
	}

	if state.reply != nil {
		return fmt.Errorf(
			"the %s already opened a session (reply status %d): this step opens exactly one session",
			role, state.reply.Status,
		)
	}

	reply, err := openSession(state.addr)
	if err != nil {
		return fmt.Errorf("the %s could not open a session: %w", role, err)
	}

	state.reply = reply

	return nil
}

// sessionIsGrantedTicket asserts the reply to initialize carries a
// ticket: 32 lowercase hexadecimal characters, on a 200.
func sessionIsGrantedTicket(state *State, _ []string) error {
	reply, err := state.openedSession()
	if err != nil {
		return err
	}

	ticket := reply.Header.Get(ticketHeader)

	if reply.Status != http.StatusOK {
		return fmt.Errorf(
			"the session was not granted a ticket: expected status 200 with a %s header of 32 lowercase hexadecimal characters, got status %d with %s: %q",
			ticketHeader, reply.Status, ticketHeader, ticket,
		)
	}

	if !regexp.MustCompile(ticketPattern).MatchString(ticket) {
		return fmt.Errorf(
			"the session was not granted a ticket: expected the %s header of the 200 reply to be 32 lowercase hexadecimal characters, got %q",
			ticketHeader, ticket,
		)
	}

	return nil
}

// serverNamesProtocolVersion asserts the reply to initialize names the
// protocol version as a YYYY-MM-DD date.
func serverNamesProtocolVersion(state *State, _ []string) error {
	reply, err := state.openedSession()
	if err != nil {
		return err
	}

	version := reply.Header.Get(protocolHeader)
	if _, perr := time.Parse(protocolLayout, version); perr != nil {
		return fmt.Errorf(
			"the server did not name its protocol version: expected the %s header of the session's opening to be a date in YYYY-MM-DD form, got %q (reply status %d)",
			protocolHeader, version, reply.Status,
		)
	}

	return nil
}
