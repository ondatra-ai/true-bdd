package steps

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// requestTimeout caps waiting for the traffic a click caused: the page fires it
// off its own render, not off the click handler.
const requestTimeout = 15 * time.Second

// requestProbeScript records every request the PAGE makes, so a clause about
// what a click caused reads the page's own traffic rather than the relay's log.
const requestProbeScript = `(() => {
  if (window.__tbddRequests) { return }
  window.__tbddRequests = []
  const inner = window.fetch
  = function (input, init) {
    const url = typeof input === "string" ? input : (input && input.url) || ""
    const method = ((init && init.method) || (input && input.method) || "GET").toUpperCase()
    let body = ""
    try { body = (init && init.body) ? String(init.body) : "" } catch (readError) { body = "" }
    window.__tbddRequests.push({ url: String(url), method: method, body: body })
    return inner.apply(this, arguments)
  }
})()`

// requestLogProbe hands the recorded requests back as JSON.
const requestLogProbe = `() => JSON.stringify(window.__tbddRequests || [])`

// pageRequest is one request the page made, as the page saw it.
type pageRequest struct {
	URL    string `json:"url"`
	Method string `json:"method"`
	Body   string `json:"body"`
}

// registerOverviewSteps binds the overview's vocabulary: what its metadata line
// carries, and what a click on one of its actions made the page ask for.
func registerOverviewSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+selectorPattern+`) contains the session's (folder|id)$`,
		assertContainsSessionField)
	// The live-read phrasing of the same measurement: what the SESSION reports
	// is the folder, so the definition is reused rather than copied.
	suite.Step(`^(`+selectorPattern+`) shows the live folder path$`, assertShowsLiveFolder)
	suite.Step(`^the page issued a session detail read$`, assertSessionDetailRead)
	suite.Step(`^the page dispatched the command "([^"]*)"$`, assertDispatchedCommand)
}

// observeRequests installs the request probe BEFORE the page navigates, which
// is the only moment at which a scenario's own traffic can all be seen.
func observeRequests(state *State, page playwright.Page) error {
	err := page.AddInitScript(playwright.Script{
		Content: playwright.String(requestProbeScript),
	})
	if err != nil {
		return state.fail("installing the request probe: %w", err)
	}

	return nil
}

// assertContainsSessionField holds an element's text to carrying what the
// registry says about this scenario's session.
func assertContainsSessionField(state *State, args []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	want := session.SessionID
	if args[1] == folderKey {
		want = session.Folder
	}

	return assertElementContainsText(state, []string{args[0], want})
}

// assertShowsLiveFolder holds an element's text to the folder the SESSION
// reports, so a page rendering a path the scenario repeated cannot pass.
func assertShowsLiveFolder(state *State, args []string) error {
	return assertContainsSessionField(state, []string{args[0], folderKey})
}

// assertSessionDetailRead holds the page to having read the session's own
// detail route since the When, rather than starting anything.
func assertSessionDetailRead(state *State, _ []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	route := sessionsPath + "/" + session.SessionID

	found, err := awaitRequest(state, func(request pageRequest) bool {
		return request.Method == http.MethodGet && strings.Contains(request.URL, route)
	})
	if err != nil {
		return err
	}

	if !found {
		return state.fail("the page issued no GET to %s; since the last click it issued %s",
			route, requestSummary(state))
	}

	return nil
}

// assertDispatchedCommand holds the page to having dispatched the command the
// clause names — recognised by the run body's own `command`, never by a route.
func assertDispatchedCommand(state *State, args []string) error {
	want := args[0]

	found, err := awaitRequest(state, func(request pageRequest) bool {
		return request.Method == http.MethodPost && dispatchedCommand(request.Body) == want
	})
	if err != nil {
		return err
	}

	if !found {
		return state.fail("the page dispatched no %q; since the last click it issued %s",
			want, requestSummary(state))
	}

	return nil
}

// dispatchedCommand is the command one body carries, or empty when the body is
// not a dispatch.
func dispatchedCommand(body string) string {
	var sent dispatchBody

	err := json.Unmarshal([]byte(body), &sent)
	if err != nil {
		return ""
	}

	return sent.Command
}

// awaitRequest waits for one of the requests made SINCE the last snapshot to be
// the one the clause is about.
func awaitRequest(state *State, matches func(pageRequest) bool) (bool, error) {
	deadline := time.Now().Add(requestTimeout)

	for {
		requests, err := requestsSince(state)
		if err != nil {
			return false, err
		}

		for _, request := range requests {
			if matches(request) {
				return true, nil
			}
		}

		if !time.Now().Before(deadline) {
			return false, nil
		}

		time.Sleep(valuePollInterval)
	}
}

// pageRequests reads every request the page has recorded so far.
func pageRequests(state *State) ([]pageRequest, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	raw, err := probeString(page, requestLogProbe)
	if err != nil {
		return nil, state.fail("reading the page's requests: %w", err)
	}

	var requests []pageRequest

	err = json.Unmarshal([]byte(raw), &requests)
	if err != nil {
		return nil, state.fail("decoding the page's requests: %w\n%s", err, raw)
	}

	return requests, nil
}

// requestCount is how much of the log a When had already seen, which is what
// keeps a later clause off the traffic that preceded it.
func requestCount(state *State) int {
	requests, err := pageRequests(state)
	if err != nil {
		return 0
	}

	return len(requests)
}

// requestsSince is the traffic recorded after that mark.
func requestsSince(state *State) ([]pageRequest, error) {
	requests, err := pageRequests(state)
	if err != nil {
		return nil, err
	}

	if state.RequestsBefore > len(requests) {
		return requests, nil
	}

	return requests[state.RequestsBefore:], nil
}

// requestSummary renders what the page DID ask for, so a failure carries the
// alternative rather than only the absence.
func requestSummary(state *State) string {
	requests, err := requestsSince(state)
	if err != nil {
		return "(the page's requests could not be read)"
	}

	if len(requests) == 0 {
		return "nothing"
	}

	lines := make([]string, 0, len(requests))
	for _, request := range requests {
		lines = append(lines, request.Method+" "+request.URL)
	}

	return strings.Join(lines, ", ")
}
