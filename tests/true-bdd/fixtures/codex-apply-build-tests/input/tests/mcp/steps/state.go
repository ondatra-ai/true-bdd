package steps

import "net/http"

// State is the existing per-Scenario host state; reuse it for definitions.
type State struct {
 BaseURL string
 Client *http.Client
 Response *http.Response
}

// Suite is the harness registration interface. Register is authored separately.
type Suite interface { Step(string, func(*State, []string) error) }
