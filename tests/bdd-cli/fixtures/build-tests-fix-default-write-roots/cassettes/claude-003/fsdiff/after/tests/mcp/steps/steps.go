//go:build bdd

// Package steps holds the step definitions of the mcp suite: every
// registry scenario whose `service:` is mcp-service binds its steps here.
package steps

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Suite is what this package needs from the harness running it: one
// registration call per step definition.
type Suite interface {
	Step(pattern string, fn func(args ...string) error)
}

// Register binds every step phrasing the mcp suite's scenarios use.
func Register(suite Suite, st *State) {
	suite.Step(`^the MCP server is running on (its configured port|port \d+)$`, func(args ...string) error {
		return givenServerRunning(st, args[0])
	})

	suite.Step(`^the (.+) posts an? (valid|malformed) JSON-RPC ([A-Za-z0-9/_.-]+) request to (/\S*)$`, func(args ...string) error {
		return whenPostsJSONRPC(st, args[0], args[1], args[2], args[3])
	})

	suite.Step(`^the server returns HTTP (\d{3})$`, func(args ...string) error {
		return thenServerReturnsStatus(st, args[0])
	})
}

// givenServerRunning prepares: it resolves the port and waits for the
// server to accept connections there.
func givenServerRunning(st *State, portPhrase string) error {
	port, err := resolvePort(portPhrase)
	if err != nil {
		return err
	}

	if err := waitForListener(port, bootBudget); err != nil {
		return err
	}

	st.Port = port
	st.BaseURL = "http://" + net.JoinHostPort(localHost, strconv.Itoa(port))

	return nil
}

// resolvePort reads either phrasing of the port: the configured one, or
// an explicit number the step names.
func resolvePort(portPhrase string) (int, error) {
	if portPhrase == configuredPortPhrase {
		return configuredPort()
	}

	raw := strings.TrimPrefix(portPhrase, "port ")

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("expected a TCP port in %q, could not read one: %w", portPhrase, err)
	}

	if port <= 0 || port > maxPort {
		return 0, fmt.Errorf("expected a TCP port in 1..%d in %q, got %d", maxPort, portPhrase, port)
	}

	return port, nil
}

// whenPostsJSONRPC acts: it posts the request the step describes and
// records what came back.
func whenPostsJSONRPC(st *State, actor, validity, method, path string) error {
	return st.postJSONRPC(actor, path, method, validity == "valid")
}

// thenServerReturnsStatus asserts the recorded status against the one the
// step names.
func thenServerReturnsStatus(st *State, want string) error {
	code, err := strconv.Atoi(want)
	if err != nil {
		return fmt.Errorf("expected a numeric HTTP status in the step, got %q: %w", want, err)
	}

	if st.LastStatus == 0 {
		return fmt.Errorf("expected HTTP %d, but nothing was sent: no When step posted to the MCP server", code)
	}

	if st.LastStatus != code {
		return fmt.Errorf("expected HTTP %d from %s's POST %s%s (JSON-RPC %q), got HTTP %d; body: %s",
			code, st.LastActor, st.BaseURL, st.LastPath, st.LastMethod, st.LastStatus, reportBody(st.LastBody))
	}

	return nil
}
