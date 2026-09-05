package steps

import (
	"errors"
	"strings"
	"time"
)

// ErrNoDispatchedRun is returned when a clause says "the dispatched run" and the
// session never held one.
var ErrNoDispatchedRun = errors.New("the session holds no dispatched run")

const (
	// runRefPattern is how a clause names a run: by the label its own dispatch gave
	// it, or as "the dispatched run" when the page dispatched it. Spliced into every
	// clause below, so one grammar change moves them all.
	runRefPattern = `run "[^"]+"|the dispatched run`
	// dispatchedLabel files a page-dispatched run, so it resolves through the same
	// Runs/RunSessions maps a named one does.
	dispatchedLabel = "the dispatched run"
	// quotedRunPrefix is how a quoted reference opens.
	quotedRunPrefix = `run "`
)

// runLabelOf resolves the label behind the reference a clause wrote.
func runLabelOf(state *State, ref string) (string, error) {
	if strings.HasPrefix(ref, quotedRunPrefix) {
		return strings.TrimSuffix(strings.TrimPrefix(ref, quotedRunPrefix), `"`), nil
	}

	return resolveDispatchedRun(state)
}

// resolveDispatchedRun files the run the page's own action created, ONCE: a later
// clause names the run the first clause resolved, not whatever the session holds
// when it is read again.
func resolveDispatchedRun(state *State) (string, error) {
	_, resolved := state.Runs[dispatchedLabel]
	if resolved {
		return dispatchedLabel, nil
	}

	session, err := ensureSession(state)
	if err != nil {
		return "", err
	}

	runID, err := awaitDispatchedRun(state, session)
	if err != nil {
		return "", err
	}

	state.Runs[dispatchedLabel] = runID
	state.RunSessions[dispatchedLabel] = session.SessionID

	return dispatchedLabel, nil
}

// awaitDispatchedRun polls the session until it holds a run and answers the one
// the page just made: its active run, else the newest it has finished. Polled,
// since the click is answered before the relay has filed the run.
func awaitDispatchedRun(state *State, session *sessionSummary) (string, error) {
	deadline := time.Now().Add(promptTimeout)

	var reason string

	for {
		detail, readErr := getSessionDetail(state, session)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case detail.ActiveRun != nil:
			return detail.ActiveRun.RunID, nil
		case len(detail.Runs) > 0:
			return detail.Runs[0].RunID, nil
		default:
			reason = "it reports no run at all"
		}

		if !time.Now().Before(deadline) {
			return "", state.fail("%w within %s: %s", ErrNoDispatchedRun, promptTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}
