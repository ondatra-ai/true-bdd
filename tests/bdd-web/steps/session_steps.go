package steps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// ErrNoSession is returned when a clause reads the session's registry
// entry and no earlier Then step resolved one.
var ErrNoSession = errors.New("no Then step resolved the session")

// ErrSessionNotListed is returned while the relay's registry does not hold
// the scenario's remote.
var ErrSessionNotListed = errors.New("the relay does not list the session")

// ErrSessionsRead is returned when the registry read itself failed.
var ErrSessionsRead = errors.New("reading the relay's session registry failed")

// ErrNoSymlink is returned when a clause is about the symlink path and no
// Given step started the remote through one.
var ErrNoSymlink = errors.New("no Given step started the remote through a symlink")

const (
	// sessionsPath is the relay's registry — the same read the sessions
	// list itself polls.
	sessionsPath = "/api/sessions"
	// sessionRowTestID and sessionFolderTestID are the sessions-list
	// selector contract: a row per connected session carrying its id, and a
	// folder cell holding the canonical path.
	sessionRowTestID    = "session-row"
	sessionFolderTestID = "session-folder"
	// sessionAppearTimeout is how long the relay has to list a remote that
	// has started: registering is a round trip no step awaits.
	sessionAppearTimeout = 30 * time.Second
	// sessionGoneTimeout is how long the relay has to DROP a remote that
	// stopped answering — the parked suite's SESSION_GONE_TIMEOUT_MS.
	sessionGoneTimeout = 30 * time.Second
	// sessionPollInterval is how often the registry is re-read.
	sessionPollInterval = 500 * time.Millisecond
	// sessionReadTimeout caps one registry read.
	sessionReadTimeout = 5 * time.Second
	// rowTimeout caps waiting for a row to render: the list is polled, so a
	// row can appear a poll after the registry holds it.
	rowTimeout = 15_000 // milliseconds
)

// sessionSummary is the registry-only shape GET /api/sessions returns per
// connected remote — the three fields the session clauses read.
type sessionSummary struct {
	SessionID string `json:"session_id"`
	Folder    string `json:"folder"`
	PID       int    `json:"pid"`
	// Version is the build the remote reported at register, which the row's
	// version cell is graded against.
	Version string `json:"version"`
	// ConnectedAt is when the registry first held this session; the restart
	// clause holds it to being the same instant after the process died.
	ConnectedAt int64 `json:"connected_at"`
}

// prepareProjectTree materializes the named fixture, so the steps after it
// have a host folder to run a remote in.
func prepareProjectTree(state *State, args []string) error {
	name := args[0]

	tree, err := materializeTree(state.Harness, state.Scenario.ID, name)
	if err != nil {
		return state.fail("preparing the %q project tree: %w", name, err)
	}

	state.Tree = tree

	before, err := readAllowedDocuments(state)
	if err != nil {
		return err
	}

	state.TreeDocsBefore = before

	return nil
}

// startRemoteThroughSymlink starts the remote with its working directory
// set to a symlink pointing at the project tree — the setup the canonical
// folder contract is asserted against.
func startRemoteThroughSymlink(state *State, _ []string) error {
	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	link, err := state.Tree.linkThrough()
	if err != nil {
		return state.fail("%w", err)
	}

	return attachRemote(state, link)
}

// startRemoteInTree starts the remote in the project tree itself — the
// plain Given a scenario opens with when no clause is about the path it
// was reached through.
func startRemoteInTree(state *State, _ []string) error {
	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	return attachRemote(state, state.Tree.Dir)
}

// attachRemote starts a remote in dir, hands its lifetime to the
// scenario's cleanup, and makes it the remote every session clause reads.
func attachRemote(state *State, dir string, env ...string) error {
	remote, err := launchRemote(state, dir, env...)
	if err != nil {
		return err
	}

	state.Remote = remote

	return nil
}

// launchRemote starts a remote in dir and hands its lifetime to the
// scenario's cleanup, without deciding which vocabulary owns it.
func launchRemote(state *State, dir string, env ...string) (*Remote, error) {
	remote, err := startRemote(state.Harness, state.RelayURL, dir, env...)
	if err != nil {
		return nil, state.fail("%w", err)
	}

	state.T.Cleanup(remote.stop)

	return remote, nil
}

// stopRemoteWithSignal is the When of a disconnect scenario: the named
// signal reaches the remote, so a frozen one's in-flight poll can never be
// renewed.
func stopRemoteWithSignal(state *State, args []string) error {
	if state.Remote == nil {
		return state.fail("%w", ErrNoRemote)
	}

	// Resolved before the signal: a remote frozen before it registered leaves
	// every later clause asserting about a session that never existed.
	_, err := ensureSession(state)
	if err != nil {
		return err
	}

	name := args[0]

	sig, err := signalNamed(name)
	if err != nil {
		return state.fail("%w", err)
	}

	err = state.Remote.signal(sig)
	if err != nil {
		return state.fail("stopping the remote (pid %d) with %s: %w",
			state.Remote.PID, name, err)
	}

	return nil
}

// assertSessionListed waits for the relay's registry to hold the remote
// and remembers the entry later clauses read. Registry-level, not
// rendered: scenarios assert it from pages with no sessions list.
func assertSessionListed(state *State, _ []string) error {
	_, err := ensureSession(state)

	return err
}

// ensureSession resolves the scenario's session once and keeps it: the
// clauses after a disconnect name a session the registry no longer holds,
// so re-reading it there would answer the wrong question.
func ensureSession(state *State) (*sessionSummary, error) {
	if state.Session != nil {
		return state.Session, nil
	}

	if state.Remote == nil {
		return nil, state.fail("%w", ErrNoRemote)
	}

	session, err := awaitSession(state, state.Remote)
	if err != nil {
		return nil, err
	}

	state.Session = session

	return session, nil
}

// awaitSession waits for the relay's registry to list one remote — the poll
// the unlabelled session and a labelled one both resolve through.
func awaitSession(state *State, remote *Remote) (*sessionSummary, error) {
	return awaitRegistered(state, fmt.Sprintf("the remote (pid %d)", remote.PID),
		func(entry *sessionSummary) bool { return entry.PID == remote.PID })
}

// awaitRegistered polls the registry until one entry matches, naming what it
// was hunting when it gives up — the poll a first resolution and a re-read
// after a restart share.
func awaitRegistered(state *State, subject string,
	matches func(*sessionSummary) bool,
) (*sessionSummary, error) {
	deadline := time.Now().Add(sessionAppearTimeout)
	last := ErrSessionNotListed

	for time.Now().Before(deadline) {
		sessions, err := listSessions(state.RelayURL)
		if err != nil {
			last = err
		}

		for index := range sessions {
			if matches(&sessions[index]) {
				return &sessions[index], nil
			}
		}

		time.Sleep(sessionPollInterval)
	}

	return nil, state.fail("the relay never listed %s within %s: %w",
		subject, sessionAppearTimeout, last)
}

// assertSessionNotListed waits for the registry to DROP the remote: every
// listed session is connected by definition, so one that stopped answering
// leaves rather than lingering as unreachable.
func assertSessionNotListed(state *State, _ []string) error {
	if state.Remote == nil {
		return state.fail("%w", ErrNoRemote)
	}

	deadline := time.Now().Add(sessionGoneTimeout)
	reason := "the relay still lists it"

	for time.Now().Before(deadline) {
		_, err := findSession(state.RelayURL, state.Remote.PID)

		// Only an absent entry is gone; a failed registry read is a failed
		// read, and keeps the poll running.
		switch {
		case errors.Is(err, ErrSessionNotListed):
			return nil
		case err != nil:
			reason = err.Error()
		default:
			reason = "the relay still lists it"
		}

		time.Sleep(sessionPollInterval)
	}

	return state.fail("the remote (pid %d) never left the registry within %s: %s",
		state.Remote.PID, sessionGoneTimeout, reason)
}

// findSession reads the registry once and picks out the entry the
// scenario's remote registered, identified by pid.
func findSession(baseURL string, pid int) (*sessionSummary, error) {
	sessions, err := listSessions(baseURL)
	if err != nil {
		return nil, err
	}

	for index := range sessions {
		if sessions[index].PID == pid {
			return &sessions[index], nil
		}
	}

	return nil, fmt.Errorf("%w: pid %d among %d listed",
		ErrSessionNotListed, pid, len(sessions))
}

// listSessions reads the relay's session registry.
func listSessions(baseURL string) ([]sessionSummary, error) {
	request, err := http.NewRequestWithContext(context.Background(),
		http.MethodGet, baseURL+sessionsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("build the %s request: %w", sessionsPath, err)
	}

	client := &http.Client{Timeout: sessionReadTimeout}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", sessionsPath, err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: GET %s returned %d",
			ErrSessionsRead, sessionsPath, response.StatusCode)
	}

	var body struct {
		Sessions []sessionSummary `json:"sessions"`
	}

	err = json.NewDecoder(response.Body).Decode(&body)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", sessionsPath, err)
	}

	return body.Sessions, nil
}

// assertSessionFolderIsRealPath is the API half of the symlink contract:
// the registry holds the canonical folder, whatever path the remote was
// started through.
func assertSessionFolderIsRealPath(state *State, _ []string) error {
	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	if state.Session == nil {
		return state.fail("%w", ErrNoSession)
	}

	if state.Session.Folder != state.Tree.Dir {
		return state.fail("the session's folder is %q, want the project tree's real path %q",
			state.Session.Folder, state.Tree.Dir)
	}

	return nil
}

// sessionRow waits for the listed session's row on the sessions list and
// returns it, or names the step that should have established it.
func sessionRow(state *State) (playwright.Locator, error) {
	page, err := state.page()
	if err != nil {
		return nil, err
	}

	if state.Tree == nil {
		return nil, state.fail("%w", ErrNoProjectTree)
	}

	if state.Session == nil {
		return nil, state.fail("%w", ErrNoSession)
	}

	row := page.Locator(fmt.Sprintf("[data-testid=%q][data-session-id=%q]",
		sessionRowTestID, state.Session.SessionID))

	err = row.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(rowTimeout),
	})
	if err != nil {
		return nil, state.fail("the sessions list never showed a row for session %q: %w\n%s",
			state.Session.SessionID, err, visibleText(page))
	}

	return row, nil
}

// assertRowShowsRealPath is the rendered half of the same contract: the
// row's folder cell carries the canonical path the registry holds.
func assertRowShowsRealPath(state *State, _ []string) error {
	row, err := sessionRow(state)
	if err != nil {
		return err
	}

	cell := row.GetByTestId(sessionFolderTestID).First()

	err = cell.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(rowTimeout),
	})
	if err != nil {
		return state.fail("the session's row never showed a folder cell: %w", err)
	}

	got, err := cell.InnerText()
	if err != nil {
		return state.fail("read the session's folder cell: %w", err)
	}

	got = strings.TrimSpace(got)
	if got != state.Tree.Dir {
		return state.fail("the session's row shows folder %q, want the project tree's real path %q",
			got, state.Tree.Dir)
	}

	return nil
}

// assertRowHidesSymlinkPath is the negative twin: the path the remote was
// started through appears nowhere in its row.
func assertRowHidesSymlinkPath(state *State, _ []string) error {
	row, err := sessionRow(state)
	if err != nil {
		return err
	}

	if state.Tree.Link == "" {
		return state.fail("%w", ErrNoSymlink)
	}

	text, err := row.InnerText()
	if err != nil {
		return state.fail("read the session's row: %w", err)
	}

	if strings.Contains(text, state.Tree.Link) {
		return state.fail("the session's row shows the symlink path %q:\n%s",
			state.Tree.Link, text)
	}

	return nil
}
