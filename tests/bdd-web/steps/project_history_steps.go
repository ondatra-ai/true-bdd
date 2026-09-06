package steps

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrRunNotInHistory is returned when no session connected to the scenario's
// project tree lists the run a clause names.
var ErrRunNotInHistory = errors.New("the project's history holds no such run")

// registerProjectHistorySteps binds what outlives the process that made it: the
// run a replacement CLI reads back, and the command it accepts afterwards.
func registerProjectHistorySteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the project history holds run "([^"]+)"$`, assertHistoryHoldsRun)
	suite.Step(
		`^dispatching "([^"]+)" on session "([^"]+)" reaches a terminal state `+
			`with outcome "([^"]*)"$`,
		assertDispatchOnSessionOutcome)
}

// assertHistoryHoldsRun holds the project's own history to carrying the run,
// read through whichever session is connected NOW: the remote that made it is
// gone, which is the whole point of the clause.
func assertHistoryHoldsRun(state *State, args []string) error {
	label := args[0]

	runID, err := lookupID(state, label)
	if err != nil {
		return err
	}

	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	deadline := time.Now().Add(reportTimeout)

	for {
		held, reason := historyHolds(state, runID)
		if held {
			return nil
		}

		if !time.Now().Before(deadline) {
			return state.fail("%w: run %q (%s) within %s: %s",
				ErrRunNotInHistory, label, runID, reportTimeout, reason)
		}

		time.Sleep(sessionPollInterval)
	}
}

// historyHolds answers whether any session connected to this scenario's tree
// lists the run, and what those sessions DID hold when none does.
func historyHolds(state *State, runID string) (bool, string) {
	sessions, err := listSessions(state.RelayURL)
	if err != nil {
		return false, err.Error()
	}

	seen := []string{}

	for index := range sessions {
		if sessions[index].Folder != state.Tree.Dir {
			continue
		}

		detail, readErr := getSessionDetail(state, &sessions[index])
		if readErr != nil {
			continue
		}

		for _, run := range detail.Runs {
			if run.RunID == runID {
				return true, ""
			}

			seen = append(seen, run.RunID)
		}
	}

	return false, "the sessions connected to " + state.Tree.Dir + " hold " + listOrNone(seen)
}

// listOrNone renders the run ids a failure names, saying so when there are none.
func listOrNone(runIDs []string) string {
	if len(runIDs) == 0 {
		return "no run at all"
	}

	return strings.Join(runIDs, ", ")
}

// assertDispatchOnSessionOutcome sends one command to the session a scenario
// labelled and grades the ending it reaches — the clause that says a folder a
// dead holder left behind is usable again.
func assertDispatchOnSessionOutcome(state *State, args []string) error {
	command, sessionLabel, want := args[0], args[1], args[2]

	session, err := ensureNamedSession(state, sessionLabel)
	if err != nil {
		return err
	}

	label := fmt.Sprintf("%s on %s", command, sessionLabel)

	err = postDispatchTo(state, state.RelayURL, session, command, label, false)
	if err != nil {
		return err
	}

	return assertRunOutcome(state, []string{label, want})
}
