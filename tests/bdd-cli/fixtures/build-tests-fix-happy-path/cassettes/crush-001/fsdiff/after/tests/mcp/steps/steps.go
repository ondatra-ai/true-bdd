package steps

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
)

// Register binds every step the mcp suite's scenarios use. Given
// prepares, When acts, Then asserts — each Then error names what was
// expected and what actually happened, because that error IS the test
// failure a reader gets.
func Register(suite *bddgo.Suite[*State]) {
	// Given — precondition. No varying token in the wording, so the
	// pattern is an anchored literal rather than a forced capture group.
	suite.Step(`^the MCP server is running on its configured port$`, func(s *State, m []string) error {
		if s.BaseURL == "" {
			s.BaseURL = "http://127.0.0.1:8080"
		}
		return nil
	})

	// When — act. Method and path vary between phrasings, so both are
	// captured: `initialize … /mcp`, `tools/list … /mcp`, …
	suite.Step(`^the Claude User posts a valid JSON-RPC (\S+) request to (\S+)$`, func(s *State, m []string) error {
		method, path := m[1], m[2]
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":{}}`, method)
		resp, err := http.Post(s.BaseURL+path, "application/json", bytes.NewBufferString(body))
		if err != nil {
			return fmt.Errorf("POST %s%s (%s) failed: %w", s.BaseURL, path, method, err)
		}
		s.Response = resp
		return nil
	})

	// Then — assert. Status code varies, so it is captured.
	suite.Step(`^the server returns HTTP (\d+)$`, func(s *State, m []string) error {
		want := m[1]
		if s.Response == nil {
			return fmt.Errorf("expected HTTP %s but no response was recorded — did the When step run?", want)
		}
		if got := fmt.Sprintf("%d", s.Response.StatusCode); got != want {
			return fmt.Errorf("expected HTTP %s, got HTTP %s", want, got)
		}
		return nil
	})
}
