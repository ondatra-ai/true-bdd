// Package steps binds the mcp suite's Given/When/Then step text to Go
// step definitions. Register wires every definition onto the suite the
// harness passes in; the suite drives them per scenario.
package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Registrar is the seam the test harness satisfies: it binds an anchored
// regexp to a handler. Kept minimal and consumer-defined so any suite
// runner can satisfy it without this package importing the harness.
type Registrar interface {
	Step(pattern string, fn any)
}

// World is the state the three definitions share across one scenario:
// which server is up, its base URL, and the last response's status.
type World struct {
	server     string
	baseURL    string
	lastStatus int
}

// Register adds every step definition the mcp suite binds. This is the
// suite's single Register call site — new definitions join it here.
func Register(suite Registrar) {
	w := &World{}
	suite.Step(`^the (\S+) server is running on its configured port$`, w.serverRunning)
	suite.Step(`^the Claude User posts a valid JSON-RPC (\S+) request to (\S+)$`, w.postJSONRPC)
	suite.Step(`^the server returns HTTP (\d+)$`, w.assertStatus)
}

// serverRunning (Given) records the named server as up and resolves its
// configured base URL.
func (w *World) serverRunning(name string) error {
	w.server = name
	w.baseURL = "http://127.0.0.1:8080"
	return nil
}

// postJSONRPC (When) posts a JSON-RPC request of the given method to the
// given path and records the response status.
func (w *World) postJSONRPC(method, path string) error {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
	})
	if err != nil {
		return fmt.Errorf("encoding %s request: %w", method, err)
	}

	resp, err := http.Post(w.baseURL+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("posting %s to %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	w.lastStatus = resp.StatusCode
	return nil
}

// assertStatus (Then) fails unless the last response carried the wanted
// HTTP status, naming both sides.
func (w *World) assertStatus(code string) error {
	want, err := strconv.Atoi(code)
	if err != nil {
		return fmt.Errorf("status code %q is not a number: %w", code, err)
	}

	if w.lastStatus != want {
		return fmt.Errorf("expected HTTP %d, got %d", want, w.lastStatus)
	}

	return nil
}
