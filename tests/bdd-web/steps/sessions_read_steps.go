package steps

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoSessionsRead is returned when a clause is about the read the page must
// have issued and it issued none.
var ErrNoSessionsRead = errors.New("the page issued no sessions read")

const (
	// The modes the sessions read is answered in. gateModePass is the page's own
	// traffic, untouched; every other mode is a step's doing.
	gateModePass    = "pass"
	gateModeHold    = "hold"
	gateModeFail    = "fail"
	gateModeHang    = "hang"
	gateModeGarbage = "garbage"
	// sessionsEmptyTestID and sessionsUnavailableTestID are the two notices the
	// list renders in place of rows.
	sessionsEmptyTestID       = "sessions-empty"
	sessionsUnavailableTestID = "sessions-unavailable"
	// recoveryTimeout is how long the list has to come back once its read
	// succeeds again: it re-reads on its own clock, not on the step's.
	recoveryTimeout = 60 * time.Second
)

// registerSessionsReadSteps binds the vocabulary that answers the sessions read
// for the page: held open, failing, hanging, or answering something that is not
// a session list — and the clauses about what the list does when it is let go.
func registerSessionsReadSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the sessions read is held open$`, holdSessionsRead)
	suite.Step(`^the sessions read begins failing$`, failSessionsRead)
	suite.Step(`^the sessions read begins hanging$`, hangSessionsRead)
	suite.Step(`^the sessions read answers 200 with a body that is not a session list$`,
		corruptSessionsRead)
	suite.Step(`^the page issued a sessions read$`, assertSessionsRead)
	suite.Step(`^releasing the read with no sessions shows sessions-empty$`,
		assertReleaseShowsEmpty)
	suite.Step(`^a genuine empty answer then shows sessions-empty$`, assertReleaseShowsEmpty)
	suite.Step(`^letting the read succeed again brings the row back and clears the notice$`,
		assertReleaseRestoresRow)
}

// sessionsGateScript answers the sessions read the way the scenario's steps ask
// for, from inside the page: the read is the page's own fetch, so holding,
// failing or corrupting it is done where the page makes it.
func sessionsGateScript(mode string) string {
	return fmt.Sprintf(`(() => {
	if (window.__tbddSessionsGate) { return }
	const gate = { mode: %[1]q }
	window.__tbddSessionsGate = gate
	const inner = window.fetch
	const isRead = (url) =>
		String(url).split("?")[0].replace(/\/$/, "").endsWith(%[2]q)
	const answer = (body, status) => new Response(body,
		{ status: status, headers: { "content-type": "application/json" } })
	const released = () => new Promise((resolve) => {
		const wait = () => {
			if (gate.mode === %[3]q) { resolve() } else { setTimeout(wait, 100) }
		}
		wait()
	})
	window.fetch = function (input, init) {
		const url = typeof input === "string" ? input : (input && input.url) || ""
		const method = ((init && init.method) ||
			(input && input.method) || "GET").toUpperCase()
		if (method !== "GET" || !isRead(url) || gate.mode === %[3]q) {
			return inner.apply(this, arguments)
		}
		if (gate.mode === %[4]q) {
			const flight = inner.apply(this, arguments)
			return released().then(() => flight)
		}
		if (window.__tbddRequests) {
			window.__tbddRequests.push({ url: String(url), method: method, body: "" })
		}
		if (gate.mode === %[5]q) { return new Promise(() => {}) }
		if (gate.mode === %[6]q) {
			return Promise.resolve(answer('{"error":"the sessions read failed"}', 500))
		}
		return Promise.resolve(answer('{"not":"a session list"}', 200))
	}
})()`, mode, sessionsPath, gateModePass, gateModeHold, gateModeHang, gateModeFail)
}

// observeSessionsGate installs the gate BEFORE the page navigates, in the mode a
// Given already asked for, so the page's very first read is answered the
// scenario's way rather than the relay's.
func observeSessionsGate(state *State, page playwright.Page) error {
	err := page.AddInitScript(playwright.Script{
		Content: playwright.String(sessionsGateScript(sessionsGateMode(state))),
	})
	if err != nil {
		return state.fail("installing the sessions read gate: %w", err)
	}

	return nil
}

// sessionsGateMode is the mode the read is answered in — the page's own traffic
// until a step says otherwise.
func sessionsGateMode(state *State) string {
	if state.SessionsGate == "" {
		return gateModePass
	}

	return state.SessionsGate
}

// setSessionsGate moves the gate on the open page AND for any page opened after
// it, so a Given may hold a read before the page that will issue it exists.
func setSessionsGate(state *State, mode string) error {
	state.SessionsGate = mode

	if state.Page == nil {
		return nil
	}

	_, err := state.Page.Evaluate(fmt.Sprintf(
		`() => { if (window.__tbddSessionsGate) { window.__tbddSessionsGate.mode = %q } }`,
		mode))
	if err != nil {
		return state.fail("setting the sessions read to %s: %w", mode, err)
	}

	return nil
}

// holdSessionsRead holds the read open: the page issues it and nothing answers
// until a later clause releases it.
func holdSessionsRead(state *State, _ []string) error {
	return setSessionsGate(state, gateModeHold)
}

// failSessionsRead answers every read from now on with a 500.
func failSessionsRead(state *State, _ []string) error {
	return setSessionsGate(state, gateModeFail)
}

// hangSessionsRead answers no read at all from now on, which is the failure a
// timeout has to be made out of.
func hangSessionsRead(state *State, _ []string) error {
	return setSessionsGate(state, gateModeHang)
}

// corruptSessionsRead answers 200 with a body that is not a session list: a
// success the page cannot read as one.
func corruptSessionsRead(state *State, _ []string) error {
	return setSessionsGate(state, gateModeGarbage)
}

// assertSessionsRead holds the page to having asked the registry at all, read
// from the page's own traffic rather than the relay's log.
func assertSessionsRead(state *State, _ []string) error {
	found, err := awaitRequest(state, func(request pageRequest) bool {
		return request.Method == http.MethodGet && strings.Contains(request.URL, sessionsPath)
	})
	if err != nil {
		return err
	}

	if !found {
		return state.fail("%w; it issued %s", ErrNoSessionsRead, requestSummary(state))
	}

	return nil
}

// assertReleaseShowsEmpty lets the read answer and holds the list to THEN saying
// that nothing is connected — the claim the earlier clause held it back from.
func assertReleaseShowsEmpty(state *State, _ []string) error {
	err := setSessionsGate(state, gateModePass)
	if err != nil {
		return err
	}

	_, _, err = locateStep(state, sessionsEmptyTestID)

	return err
}

// assertReleaseRestoresRow lets the read succeed and holds the list to coming
// back on its own: the row again, and no unavailable notice left standing.
func assertReleaseRestoresRow(state *State, _ []string) error {
	err := setSessionsGate(state, gateModePass)
	if err != nil {
		return err
	}

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	row := selector{Name: sessionRowTestID, Key: sessionIDKey, Value: session.SessionID}

	err = assertRecoveredCount(state, row, "1", "its row back once the read succeeds")
	if err != nil {
		return err
	}

	return assertRecoveredCount(state, selector{Name: sessionsUnavailableTestID}, "0",
		"the notice gone once the read succeeds")
}

// assertRecoveredCount holds how many elements a selector matches to a number, on
// the budget the list's own re-read needs rather than a render's.
func assertRecoveredCount(state *State, sel selector, want, wanted string) error {
	page, err := state.page()
	if err != nil {
		return err
	}

	got, matched, err := awaitWithin(recoveryTimeout, readCount(page, sel), equals(want))
	if err != nil {
		return state.fail("%s: %w", sel, err)
	}

	if !matched {
		return state.fail("the page shows %s %s after %s, want %s (exactly %s)",
			got, sel, recoveryTimeout, wanted, want)
	}

	return nil
}
