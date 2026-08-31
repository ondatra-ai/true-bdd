//go:build bdd

// Package steps holds the executable step definitions for the `mcp`
// suite. The scenario runner hands Register a Suite, and each
// suite.Step call binds one anchored pattern to the function that runs
// that step: Given prepares, When acts, Then asserts.
package steps

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Suite is the registrar the scenario runner passes to Register: one
// Step call per definition, binding a regexp to its function.
type Suite interface {
	Step(pattern string, fn func(*State, []string) error)
}

// State is the harness state threaded through a scenario's steps.
// Given fills baseURL and client; When records resp; Then reads it.
type State struct {
	baseURL string
	client  *http.Client
	resp    *http.Response
}

// Register binds every step this suite owns. It is the suite's single
// registration site — the runner calls it once before a scenario runs.
func Register(suite Suite) {
	suite.Step(`^the (\w+) server is running on its configured port$`, givenServerRunning)
	suite.Step(`^the Claude User posts a valid JSON-RPC initialize request to (/\S+)$`, whenPostInitialize)
	suite.Step(`^the server returns HTTP (\d{3})$`, thenServerReturnsStatus)
}

// givenServerRunning confirms the named server answers on its
// configured address before the scenario acts against it.
func givenServerRunning(st *State, m []string) error {
	name := m[1]

	addr := os.Getenv("MCP_SERVER_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8080"
	}

	st.baseURL = addr
	st.client = &http.Client{Timeout: 5 * time.Second}

	resp, err := st.client.Get(st.baseURL + "/healthz")
	if err != nil {
		return fmt.Errorf("%s server not reachable at %s: %w", name, st.baseURL, err)
	}
	resp.Body.Close()

	return nil
}

// whenPostInitialize POSTs a minimal valid JSON-RPC initialize request
// to the captured path and records the response for the Then step.
func whenPostInitialize(st *State, m []string) error {
	path := m[1]
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)

	resp, err := st.client.Post(st.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST %s%s failed: %w", st.baseURL, path, err)
	}

	st.resp = resp

	return nil
}

// thenServerReturnsStatus asserts the recorded response carries the
// captured HTTP status code.
func thenServerReturnsStatus(st *State, m []string) error {
	if st.resp == nil {
		return fmt.Errorf("no response recorded: the When step did not run")
	}
	defer st.resp.Body.Close()

	want, err := strconv.Atoi(m[1])
	if err != nil {
		return fmt.Errorf("bad expected status %q: %w", m[1], err)
	}

	if st.resp.StatusCode != want {
		return fmt.Errorf("expected HTTP %d, got %d", want, st.resp.StatusCode)
	}

	return nil
}
