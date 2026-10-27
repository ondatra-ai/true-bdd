package steps

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Register binds the registry's step vocabulary to executable code.
//
// Every pattern is anchored and carries a capture group wherever the
// step's wording varies — the actor, the JSON-RPC method, the request
// path, the expected status — so one definition serves every scenario
// that phrases the step that way instead of only the one that first
// needed it.
func Register(s Suite) {
	s.Step(`^the (.+) server is running on its configured port$`, givenServerRunningOnConfiguredPort)
	s.Step(`^the (.+) posts a valid JSON-RPC ([A-Za-z0-9_./-]+) request to (/\S*)$`, whenPostsValidJSONRPCRequest)
	s.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// captures normalises the argument slice the harness hands a definition.
//
// Harnesses differ on whether the full match is prepended to the
// submatches; accept either shape rather than let that detail decide
// whether a scenario runs.
func captures(args []string, want int) ([]string, error) {
	switch len(args) {
	case want:
		return args, nil
	case want + 1:
		return args[1:], nil
	default:
		return nil, fmt.Errorf("expected %d capture(s) from the step pattern, got %d: %q", want, len(args), args)
	}
}

// serverAddress validates the per-scenario host state the suite's Init
// prepared and returns the parsed address of the server under test.
func serverAddress(st *State) (*url.URL, error) {
	if st == nil {
		return nil, fmt.Errorf("expected an initialised scenario State, got nil")
	}
	if st.BaseURL == "" {
		return nil, fmt.Errorf("expected State.BaseURL to hold the server address, got an empty string")
	}
	if st.Client == nil {
		return nil, fmt.Errorf("expected State.Client to hold an HTTP client, got nil")
	}
	address, err := url.Parse(st.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("expected State.BaseURL %q to parse as a URL, got error: %w", st.BaseURL, err)
	}
	if address.Host == "" {
		return nil, fmt.Errorf("expected State.BaseURL %q to carry a host, got none", st.BaseURL)
	}
	return address, nil
}

// givenServerRunningOnConfiguredPort prepares: it asserts the suite
// handed this scenario a reachable server and that the server is
// listening on the port the configuration names, then clears any
// response left over from an earlier step.
func givenServerRunningOnConfiguredPort(st *State, args []string) error {
	groups, err := captures(args, 1)
	if err != nil {
		return err
	}
	name := groups[0]

	address, err := serverAddress(st)
	if err != nil {
		return fmt.Errorf("expected the %s server to be running: %w", name, err)
	}

	configured := os.Getenv("MCP_PORT")
	if configured == "" {
		return fmt.Errorf("expected the %s server's configured port in MCP_PORT, got an empty value", name)
	}
	if port := address.Port(); port != configured {
		return fmt.Errorf("expected the %s server to be running on its configured port %s, got a server at %s listening on port %q", name, configured, st.BaseURL, port)
	}

	st.Response = nil
	return nil
}

// whenPostsValidJSONRPCRequest acts: it POSTs a well-formed JSON-RPC 2.0
// envelope for the captured method to the captured path and records the
// response on the shared State for the Then steps to assert against.
func whenPostsValidJSONRPCRequest(st *State, args []string) error {
	groups, err := captures(args, 3)
	if err != nil {
		return err
	}
	actor, method, path := groups[0], groups[1], groups[2]

	address, err := serverAddress(st)
	if err != nil {
		return fmt.Errorf("expected to post as %s: %w", actor, err)
	}

	endpoint := strings.TrimSuffix(address.String(), "/") + path
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":{}}`, method)

	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("expected to build the %s POST %s request for %s, got error: %w", method, endpoint, actor, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := st.Client.Do(request)
	if err != nil {
		return fmt.Errorf("expected POST %s (%s) sent by %s to reach the server, got transport error: %w", endpoint, method, actor, err)
	}

	st.Response = response
	return nil
}

// thenServerReturnsStatus asserts: the recorded response carries the
// status code the step names, and the error says which code was wanted
// and which arrived.
func thenServerReturnsStatus(st *State, args []string) error {
	groups, err := captures(args, 1)
	if err != nil {
		return err
	}
	want, err := strconv.Atoi(groups[0])
	if err != nil {
		return fmt.Errorf("expected a numeric HTTP status in the step text, got %q: %w", groups[0], err)
	}

	if st == nil || st.Response == nil {
		return fmt.Errorf("expected HTTP %d, got no recorded response: no request step ran before this assertion", want)
	}
	if st.Response.StatusCode != want {
		return fmt.Errorf("expected HTTP %d, got HTTP %d (%q)", want, st.Response.StatusCode, st.Response.Status)
	}
	return nil
}
