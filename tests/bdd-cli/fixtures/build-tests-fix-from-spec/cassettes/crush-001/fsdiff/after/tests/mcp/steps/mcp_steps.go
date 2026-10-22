package steps

import (
	"fmt"
	"net/http"
	"regexp"
	"time"
)

const (
	// ticketHeader carries the ticket the MCP server grants a session.
	ticketHeader = "X-Session-Ticket"

	// protocolHeader carries the protocol version the server speaks.
	protocolHeader = "X-MCP-Protocol"

	// dateLayout is the YYYY-MM-DD form a protocol version takes.
	dateLayout = "2006-01-02"
)

// ticketPattern is a ticket's shape: 32 lowercase hexadecimal characters.
var ticketPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// protocolVersionPattern is a protocol version's shape: a date in
// YYYY-MM-DD form, such as 2025-06-18.
var protocolVersionPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Register lists every step definition the mcp suite binds. It is the one
// call site: a new definition belongs here, beside the others.
func Register(suite *Suite) {
	st := suite.State()

	// given: `the MCP server is running`
	suite.Step(`^the MCP server is running$`, func([]string) error {
		return st.WaitReady()
	})

	// when: `the {actor:role} opens a session` — the role varies from one
	// scenario to the next, so it is captured instead of spelled out.
	suite.Step(`^the (.+) opens a session$`, func(args []string) error {
		if len(args) != 1 || args[0] == "" {
			return fmt.Errorf("the step named no role whose client opens the session")
		}
		st.Actor = args[0]
		return st.OpenSession()
	})

	// then: `the session is granted a ticket`
	suite.Step(`^the session is granted a ticket$`, func([]string) error {
		reply, err := st.OpeningReply()
		if err != nil {
			return err
		}
		ticket := reply.Header.Get(ticketHeader)
		if reply.Status != http.StatusOK {
			return fmt.Errorf(
				"the session the %s opened was not accepted: expected status 200 from the reply to initialize, got %d (%s: %q; body: %q)",
				st.ActorName(), reply.Status, ticketHeader, ticket, reply.Snippet())
		}
		if ticket == "" {
			return fmt.Errorf(
				"the session the %s opened was granted no ticket: the 200 reply to initialize carries no %s header (headers: %v)",
				st.ActorName(), ticketHeader, headerNames(reply.Header))
		}
		if !ticketPattern.MatchString(ticket) {
			return fmt.Errorf(
				"malformed ticket: expected 32 lowercase hexadecimal characters in %s, got %q (%d characters) on a %d reply",
				ticketHeader, ticket, len(ticket), reply.Status)
		}
		return nil
	})

	// then: `the server names its protocol version`
	suite.Step(`^the server names its protocol version$`, func([]string) error {
		reply, err := st.OpeningReply()
		if err != nil {
			return err
		}
		version := reply.Header.Get(protocolHeader)
		if version == "" {
			return fmt.Errorf(
				"the server named no protocol version: the %d reply to initialize carries no %s header (headers: %v)",
				reply.Status, protocolHeader, headerNames(reply.Header))
		}
		if !protocolVersionPattern.MatchString(version) {
			return fmt.Errorf(
				"malformed protocol version: expected a date in YYYY-MM-DD form in %s, got %q on a %d reply",
				protocolHeader, version, reply.Status)
		}
		if _, err := time.Parse(dateLayout, version); err != nil {
			return fmt.Errorf(
				"malformed protocol version: %s carries %q, which is not a calendar date in YYYY-MM-DD form: %w",
				protocolHeader, version, err)
		}
		return nil
	})
}

// headerNames lists the header names a reply carried, so an error about a
// missing header shows what was there instead.
func headerNames(h http.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	return names
}
