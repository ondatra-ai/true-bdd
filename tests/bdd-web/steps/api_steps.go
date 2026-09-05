package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrNoResponse is returned when a clause reads the last relay response
// and no earlier step made a request.
var ErrNoResponse = errors.New("no earlier step made an API request")

// ErrDispatchRefused is returned when the relay did not accept a dispatch
// the scenario states as a Given.
var ErrDispatchRefused = errors.New("the relay refused the dispatch")

const (
	// apiTimeout caps one browser-family request. The relay bounds a
	// mutation at ten seconds of its own, so past this the request never
	// arrived rather than the relay being slow.
	apiTimeout = 15 * time.Second
	// bodySnippet caps how much of a body a failure quotes, so a page-sized
	// error cannot bury the status it is explaining.
	bodySnippet = 300
	// sessionPlaceholder is what a scenario writes for the session its
	// remote registered: {S}.
	sessionPlaceholder = "S"
)

// apiResponse is one browser-family response: what a status clause
// compares, and what the clause after it reads the error name out of.
type apiResponse struct {
	Status int
	Body   []byte
}

// snippet renders the body a failure quotes.
func (response *apiResponse) snippet() string {
	if len(response.Body) > bodySnippet {
		return string(response.Body[:bodySnippet]) + "…"
	}

	return string(response.Body)
}

// dispatchBody is the POST /api/sessions/:id/runs body: the command, the
// fix flag a protocol dispatch leaves false, and the relay's idempotency
// key.
type dispatchBody struct {
	Command string `json:"command"`
	// StoryID is set only by a story command; omitted otherwise, so a dispatch
	// that names no story is the byte-identical body it always was.
	StoryID     string `json:"story_id,omitempty"`
	Fix         bool   `json:"fix"`
	ClientToken string `json:"client_token"`
}

// dispatchRun sends the dispatch the UI sends and records the run id under
// the label the scenario named it by, so a later clause asks for it as
// {R1}.
func dispatchRun(state *State, args []string) error {
	command, label := args[1], args[2]

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	return postDispatch(state, session, command, label)
}

// postDispatch sends the dispatch the UI sends to one session and records the
// run under the label the scenario named it by — the request the unlabelled
// clause and the named-session one both make.
func postDispatch(state *State, session *sessionSummary, command, label string) error {
	return postDispatchTo(state, state.RelayURL, session, command, label, false)
}

// recordRun holds the relay to accepting the dispatch and remembers the
// run id it created.
func recordRun(state *State, command, label string, response *apiResponse) error {
	if response.Status != http.StatusCreated && response.Status != http.StatusOK {
		return state.fail("%w: dispatching %q returned %d, want 201 or 200: %s",
			ErrDispatchRefused, command, response.Status, response.snippet())
	}

	var accepted struct {
		RunID string `json:"run_id"`
	}

	err := json.Unmarshal(response.Body, &accepted)
	if err != nil {
		return state.fail("decode the dispatch of %q: %w\n%s",
			command, err, response.snippet())
	}

	if accepted.RunID == "" {
		return state.fail("the dispatch of %q named no run_id: %s",
			command, response.snippet())
	}

	state.Runs[label] = accepted.RunID

	return nil
}

// assertGetStatus reads one relay endpoint and holds it to a status. The
// path carries the scenario's placeholders, so one definition serves the
// session read, the status view and the run read.
func assertGetStatus(state *State, args []string) error {
	path, err := resolveIDs(state, args[0])
	if err != nil {
		return err
	}

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w",
			args[1], err)
	}

	response, err := apiGet(state.RelayURL, path)
	if err != nil {
		return state.fail("%w", err)
	}

	state.Response = response

	if response.Status != want {
		return state.fail("GET %s returned %d, want %d: %s",
			path, response.Status, want, response.snippet())
	}

	return nil
}

// assertDispatchStatus sends the dispatch the UI sends and holds the relay
// to a status — the clause a scenario writes when the refusal IS the
// outcome, so there is no accepted run to label.
func assertDispatchStatus(state *State, args []string) error {
	path, err := dispatchPath(state, args[0])
	if err != nil {
		return err
	}

	command := args[1]

	want, err := strconv.Atoi(args[2])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w",
			args[2], err)
	}

	response, err := apiPostJSON(state.RelayURL, path, dispatchBody{
		Command:     command,
		Fix:         false,
		ClientToken: probeToken(state, command),
	})
	if err != nil {
		return state.fail("%w", err)
	}

	state.Response = response

	if response.Status != want {
		return state.fail("POST %s dispatching %q returned %d, want %d: %s",
			path, command, response.Status, want, response.snippet())
	}

	return nil
}

// dispatchPath resolves the step's path, asking the registry for the
// session first when the step names {S}: a refusal clause stands on a Given
// remote no earlier step has resolved a session for.
func dispatchPath(state *State, raw string) (string, error) {
	if strings.Contains(raw, "{"+sessionPlaceholder+"}") {
		_, err := ensureSession(state)
		if err != nil {
			return "", err
		}
	}

	return resolveIDs(state, raw)
}

// probeToken is this clause's idempotency token: its own `probe-` namespace,
// so it never dedups against a labelled dispatch, and the command sanitized,
// so a shell fragment cannot make the relay refuse the TOKEN instead.
func probeToken(state *State, command string) string {
	safe := strings.Map(func(char rune) rune {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			return char
		}

		return '-'
	}, strings.ToLower(command))

	return fmt.Sprintf("e2e-%s-probe-%s", state.Scenario.ID, safe)
}

// assertResponseError reads the error name off the last response — the
// clause that turns a bare 404 into the mapped reason the relay owes the
// browser.
func assertResponseError(state *State, args []string) error {
	if state.Response == nil {
		return state.fail("%w", ErrNoResponse)
	}

	want := args[0]

	var body struct {
		Error string `json:"error"`
	}

	err := json.Unmarshal(state.Response.Body, &body)
	if err != nil {
		return state.fail("the response body is not JSON: %w\n%s",
			err, state.Response.snippet())
	}

	if body.Error != want {
		return state.fail("the response names error %q, want %q: %s",
			body.Error, want, state.Response.snippet())
	}

	return nil
}

// resolveIDs substitutes the placeholders a scenario writes into a path:
// {S} is the session its remote registered, {R1} a run an earlier dispatch
// created.
func resolveIDs(state *State, path string) (string, error) {
	var failure error

	// Compiled per call: a package-level regexp is a global, and a step runs
	// a handful of times.
	placeholder := regexp.MustCompile(`\{[^{}]+\}`)

	resolved := placeholder.ReplaceAllStringFunc(path, func(token string) string {
		value, err := lookupID(state, strings.Trim(token, "{}"))
		if err != nil && failure == nil {
			failure = err
		}

		return value
	})

	if failure != nil {
		return "", failure
	}

	return resolved, nil
}

// lookupID answers what one placeholder stands for, or names every id the
// scenario has recorded so far.
func lookupID(state *State, name string) (string, error) {
	if name == sessionPlaceholder {
		if state.Session == nil {
			return "", state.fail("%w", ErrNoSession)
		}

		return state.Session.SessionID, nil
	}

	runID, ok := state.Runs[name]
	if !ok {
		return "", state.fail("the scenario recorded no id for {%s}; it has %s",
			name, strings.Join(recordedIDs(state), ", "))
	}

	return runID, nil
}

// recordedIDs lists the placeholders the scenario can resolve, so a
// failure names what it has rather than only what it wanted.
func recordedIDs(state *State) []string {
	names := make([]string, 0, len(state.Runs)+1)

	if state.Session != nil {
		names = append(names, "{"+sessionPlaceholder+"}")
	}

	for label := range state.Runs {
		names = append(names, "{"+label+"}")
	}

	if len(names) == 0 {
		return []string{noneWord}
	}

	sort.Strings(names)

	return names
}

// apiGet reads one relay endpoint, query and all.
func apiGet(baseURL, path string) (*apiResponse, error) {
	request, err := http.NewRequestWithContext(context.Background(),
		http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build the GET %s request: %w", path, err)
	}

	return sendAPI(request, baseURL, path)
}

// apiPostJSON sends a browser-family mutation: a JSON body under the exact
// allowed Origin, which is what the relay admits and nothing else.
func apiPostJSON(baseURL, path string, payload any) (*apiResponse, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode the POST %s body: %w", path, err)
	}

	request, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build the POST %s request: %w", path, err)
	}

	request.Header.Set("Content-Type", "application/json")

	return sendAPI(request, baseURL, path)
}

// sendAPI stamps the origin the UI's own fetch calls carry, sends, and
// reads the whole body: a status clause and the error clause after it read
// one response.
func sendAPI(request *http.Request, baseURL, path string) (*apiResponse, error) {
	request.Header.Set("Origin", baseURL)

	client := &http.Client{Timeout: apiTimeout}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", request.Method, path, err)
	}

	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read the %s %s response: %w", request.Method, path, err)
	}

	return &apiResponse{Status: response.StatusCode, Body: body}, nil
}
