package steps

import (
	"fmt"
	"net/http"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// answerableVerdict is what a clause calls a run it may answer.
const answerableVerdict = "answerable"

// registerCrossSessionAnswerSteps binds the ownership vocabulary: a run read and
// answered through a session that is not the one that owns it.
func registerCrossSessionAnswerSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) answers the pending prompt `+
			`of run "([^"]+)" through session "([^"]+)" with "([^"]*)"$`,
		answerThroughSession)
	suite.Step(`^run "([^"]+)" is (answerable|not answerable) through session "([^"]+)"$`,
		assertAnswerableThroughSession)
	suite.Step(
		`^answering the pending prompt of run "([^"]+)" through session "([^"]+)" `+
			`with "([^"]*)" advances it past the prompt$`,
		assertAnswerThroughSessionAdvances)
}

// answerThroughSession answers a run's prompt on the route of the session the
// step names. The status is recorded rather than graded — the scenario's own
// clauses say what the relay owed. args[0] is the role, discarded.
func answerThroughSession(state *State, args []string) error {
	return answerVia(state, args[1], args[2], args[3])
}

// assertAnswerThroughSessionAdvances is the same answer held to being ACCEPTED
// and consumed: the run must leave the prompt it was blocked on.
func assertAnswerThroughSessionAdvances(state *State, args []string) error {
	runLabel, sessionLabel, value := args[0], args[1], args[2]

	err := answerVia(state, runLabel, sessionLabel, value)
	if err != nil {
		return err
	}

	if state.Answer.Status != http.StatusOK {
		return state.fail("answering run %q through session %q with %q returned %d, want 200: %s",
			runLabel, sessionLabel, value, state.Answer.Status, state.Answer.snippet())
	}

	return assertRunAdvancedPastPrompt(state, []string{runLabel})
}

// assertAnswerableThroughSession reads the run THROUGH the session the step
// names: answerable is owner-relative, so the same run reads differently on a
// session that only watches it.
func assertAnswerableThroughSession(state *State, args []string) error {
	runLabel, verdict, sessionLabel := args[0], args[1], args[2]
	want := verdict == answerableVerdict

	path, err := crossSessionRunPath(state, runLabel, sessionLabel)
	if err != nil {
		return err
	}

	detail, err := readRun(state, path)
	if err != nil {
		return err
	}

	if detail.Answerable != want {
		return state.fail("run %q read through session %q reads answerable=%t, want %t "+
			"(it is %q)", runLabel, sessionLabel, detail.Answerable, want, detail.State)
	}

	return nil
}

// answerVia waits for the run to block on a prompt and answers THAT prompt on
// the named session's route, keeping what the relay said for the clause after.
func answerVia(state *State, runLabel, sessionLabel, value string) error {
	prompt, err := awaitPendingPrompt(state, runLabel)
	if err != nil {
		return err
	}

	rememberPrompt(state, runLabel, prompt.PromptID)

	path, err := crossSessionRunPath(state, runLabel, sessionLabel)
	if err != nil {
		return err
	}

	response, err := apiPostJSON(state.RelayURL, path+answerRoute,
		answerBody{PromptID: prompt.PromptID, Value: value})
	if err != nil {
		return state.fail("answering prompt %q of run %q through session %q: %w",
			prompt.PromptID, runLabel, sessionLabel, err)
	}

	state.Response = response
	state.Answer = response

	return nil
}

// crossSessionRunPath is a run's route UNDER the session a step names, which is
// deliberately not the run's own owner in the clauses this file is for.
func crossSessionRunPath(state *State, runLabel, sessionLabel string) (string, error) {
	runID, err := lookupID(state, runLabel)
	if err != nil {
		return "", err
	}

	session, err := ensureNamedSession(state, sessionLabel)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/runs/%s", sessionsPath, session.SessionID, runID), nil
}
