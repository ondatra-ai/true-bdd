package steps

import (
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerSessionRunSteps binds what the session says about its own work once a
// run is over — the clause that says the reader can start another straight away.
func registerSessionRunSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the session has no active run$`, assertNoActiveRun)
}

// assertNoActiveRun holds the session to reporting no run of its own. Polled: the
// relay learns a run ended on the remote's next answer, so the first read after
// the run reached its terminal state is a read before the session knows.
func assertNoActiveRun(state *State, _ []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(reportTimeout)

	var reason string

	for {
		detail, readErr := getSessionDetail(state, session)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case detail.ActiveRun == nil:
			return nil
		default:
			reason = describeReport(detail)
		}

		if !time.Now().Before(deadline) {
			return state.fail("the session still reports an active run after %s: %s",
				reportTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}
