package steps

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerAIDispatchSteps binds the dispatch the AI scenarios open with: a
// command sent with the fix flag, optionally naming its story and session. All
// three are captured, so one definition serves us-refine, us-apply and both builds.
func registerAIDispatchSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`for story "([^"]+)" with fix as run "([^"]+)"$`,
		dispatchStoryFixRun)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`with fix as run "([^"]+)"$`,
		dispatchFixRun)
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) dispatches "([^"]+)" `+
			`for story "([^"]+)" with fix on session "([^"]+)" as run "([^"]+)"$`,
		dispatchStoryFixRunOnSession)
}

// dispatchStoryFixRun sends a story command with fix on the scenario's own
// session. args[0] is the role, discarded as openPath's is.
func dispatchStoryFixRun(state *State, args []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	return dispatchFix(state, session, args[1], args[2], args[3])
}

// dispatchFixRun is the same dispatch for a command that names no story.
func dispatchFixRun(state *State, args []string) error {
	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	return dispatchFix(state, session, args[1], "", args[2])
}

// dispatchStoryFixRunOnSession sends it to the session the scenario labelled,
// which is how a two-remote scenario says which owner asked.
func dispatchStoryFixRunOnSession(state *State, args []string) error {
	session, err := ensureNamedSession(state, args[3])
	if err != nil {
		return err
	}

	return dispatchFix(state, session, args[1], args[2], args[4])
}

// dispatchFix posts one fix dispatch and files the run under its label, through
// the same sender the protocol clauses use.
func dispatchFix(state *State, session *sessionSummary, command, story, label string) error {
	return postDispatchSpec(state, state.RelayURL, session, dispatchBody{
		Command:     command,
		StoryID:     story,
		Fix:         true,
		ClientToken: dispatchToken(state, label),
	}, label)
}
