package steps

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoRestart is returned when a clause compares the registry across a
// restart and no When step restarted the relay.
var ErrNoRestart = errors.New("no When step restarted the relay")

// ErrNoConnectTime is returned when the registry entry carries no
// connected_at, which would leave "unchanged" comparing two zeroes.
var ErrNoConnectTime = errors.New("the registry entry carries no connected_at")

// registerRestartSteps binds the restart-survival vocabulary: the process
// serving the registry dies and comes back on the same port, and what the
// registry then says about the session it was serving.
func registerRestartSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the relay restarts$`, restartRelay)
	suite.Step(`^the session's id, pid and connect time are unchanged$`,
		assertSessionUnchanged)
	suite.Step(`^run "([^"]+)" still waited on the same prompt after the restart$`,
		assertPromptSurvivedRestart)
}

// restartRelay kills the relay this scenario's clauses are about and starts
// another on the same port. The session is snapshotted BEFORE the kill: the
// entry the clause after it compares has to be the one the dead process listed.
func restartRelay(state *State, _ []string) error {
	err := rememberSessionBeforeRestart(state)
	if err != nil {
		return err
	}

	state.PromptsBeforeRestart = maps.Clone(state.Prompts)

	for label := range state.Prompts {
		noteCommandChild(state, label, state.ChildBeforeRestart)
	}

	if state.Relay != nil {
		err = state.Relay.restart(context.Background())
	} else {
		err = state.Harness.Restart(context.Background())
	}

	if err != nil {
		return state.fail("restarting the relay at %s: %w", state.RelayURL, err)
	}

	return nil
}

// rememberSessionBeforeRestart snapshots the entry a remote registered. A
// scenario running no remote of its own (an agent this suite registered) has
// nothing to snapshot, and that is not a failure.
func rememberSessionBeforeRestart(state *State) error {
	if state.Remote == nil {
		return nil
	}

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	state.SessionBefore = session

	return nil
}

// assertSessionUnchanged re-reads the registry after the restart and holds it
// to the identity the dead process listed: a registry that outlives its process
// hands back the same session, not an equivalent one.
func assertSessionUnchanged(state *State, _ []string) error {
	before := state.SessionBefore
	if before == nil {
		return state.fail("%w", ErrNoRestart)
	}

	if before.ConnectedAt == 0 {
		return state.fail("%w: session %q", ErrNoConnectTime, before.SessionID)
	}

	after, err := awaitRegistered(state, fmt.Sprintf("session %q", before.SessionID),
		func(entry *sessionSummary) bool { return entry.SessionID == before.SessionID })
	if err != nil {
		return err
	}

	if after.PID != before.PID || after.ConnectedAt != before.ConnectedAt {
		return state.fail(
			"after the restart session %q is pid %d connected at %d, "+
				"want pid %d connected at %d",
			before.SessionID, after.PID, after.ConnectedAt,
			before.PID, before.ConnectedAt)
	}

	return nil
}

// assertPromptSurvivedRestart holds the run to the prompt it was blocked on when
// the relay went down: a restart that minted a fresh id would be answered by a
// reader whose click names a prompt the run no longer holds.
func assertPromptSurvivedRestart(state *State, args []string) error {
	label := args[0]

	if state.PromptsBeforeRestart == nil {
		return state.fail("%w", ErrNoRestart)
	}

	before, seen := state.PromptsBeforeRestart[label]
	if !seen {
		return state.fail("%w: run %q was on no prompt when the relay restarted",
			ErrNoPrompt, label)
	}

	after := state.Prompts[label]
	if after != before {
		return state.fail("run %q waited on prompt %q after the restart, want the %q it "+
			"was blocked on before it", label, after, before)
	}

	return assertOnlyPromptPublished(state, label, before)
}

// assertOnlyPromptPublished holds the run's own window to naming that prompt and
// no other, which is what says the relay restored the prompt rather than
// republishing one under a new id.
func assertOnlyPromptPublished(state *State, label, promptID string) error {
	detail, err := runDetailOf(state, label)
	if err != nil {
		return err
	}

	for _, event := range detail.Events {
		if event.PromptID != "" && event.PromptID != promptID {
			return state.fail("run %q's window names prompt %q beside %q, want only the "+
				"prompt it was blocked on before the restart",
				label, event.PromptID, promptID)
		}
	}

	return nil
}
