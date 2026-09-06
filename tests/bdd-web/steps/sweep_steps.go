package steps

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoPreSweepRead is returned when a clause grades the read taken before the
// sweep and no When step took one.
var ErrNoPreSweepRead = errors.New("no When step read the session before the sweep")

const (
	// The labels the fresh-session clause files its own registration, work and
	// read under: it is one clause, and nothing else addresses them.
	freshAgentLabel   = "fresh"
	freshWorkLabel    = "fresh-work"
	freshRequestLabel = "fresh-read"
)

// registerSweepSteps binds the reclamation vocabulary: a session that stopped
// answering leaving the registry, what the relay answered while it was still
// holding capacity, and the proof that the capacity came back.
func registerSweepSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^session "([^"]+)" is swept out of the registry$`, sweepSession)
	suite.Step(
		`^a further session detail read of "([^"]+)" returned status (\d+) before the sweep$`,
		assertPreSweepRead)
	suite.Step(`^a read for a freshly registered session is enqueued and served$`,
		assertFreshReadServed)
}

// sweepSession is the When of a reclamation scenario. The further read is taken
// FIRST and its status kept, because the clause about it is past tense: it is
// about what the relay answered while the dead session still held the queue.
func sweepSession(state *State, args []string) error {
	label := args[0]

	agent, err := lookupAgent(state, label)
	if err != nil {
		return err
	}

	err = recordPreSweepRead(state, label, agent)
	if err != nil {
		return err
	}

	return awaitSwept(state, label, agent)
}

// recordPreSweepRead reads the session detail once more while the session is
// still listed, keeping the status for the clause that grades it.
func recordPreSweepRead(state *State, label string, agent *agentSession) error {
	response, err := apiGet(state.RelayURL,
		fmt.Sprintf("%s/%s", sessionsPath, agent.SessionID))
	if err != nil {
		return state.fail("%w", err)
	}

	state.PreSweep[label] = response.Status

	return nil
}

// awaitSwept waits for the registry to drop a session that stopped answering:
// every listed session is reachable by definition, so a dead one leaves rather
// than lingering with the capacity it holds.
func awaitSwept(state *State, label string, agent *agentSession) error {
	deadline := time.Now().Add(sessionGoneTimeout)

	var reason string

	for {
		entry, listed, readErr := findListed(state.RelayURL, agent.SessionID)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case entry == nil:
			return nil
		default:
			reason = fmt.Sprintf("the registry still lists it among %d session(s)", listed)
		}

		if !time.Now().Before(deadline) {
			return state.fail("session %q (%s) was never swept out of the registry "+
				"within %s: %s", label, agent.SessionID, sessionGoneTimeout, reason)
		}

		time.Sleep(sessionPollInterval)
	}
}

// assertPreSweepRead grades the status the When kept.
func assertPreSweepRead(state *State, args []string) error {
	label := args[0]

	want, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names status %q, which is not a number: %w", args[1], err)
	}

	got, probed := state.PreSweep[label]
	if !probed {
		return state.fail("%w: session %q", ErrNoPreSweepRead, label)
	}

	if got != want {
		return state.fail("the further session detail read of %q returned %d "+
			"before the sweep, want %d", label, got, want)
	}

	return nil
}

// assertFreshReadServed proves the capacity came back: a session registered
// AFTER the sweep is enqueued and answered, which a relay still holding the
// dead session's abandoned work would refuse.
func assertFreshReadServed(state *State, _ []string) error {
	err := registerAgent(state, []string{freshAgentLabel})
	if err != nil {
		return err
	}

	agent, err := lookupAgent(state, freshAgentLabel)
	if err != nil {
		return err
	}

	state.Requests[freshRequestLabel] = sendPendingRead(state.RelayURL,
		fmt.Sprintf("%s/%s", sessionsPath, agent.SessionID), agent.SessionID)

	err = serveFreshRead(state, agent)
	if err != nil {
		return err
	}

	return assertRequestCompletes(state,
		[]string{freshRequestLabel, strconv.Itoa(http.StatusOK)})
}

// serveFreshRead polls as the fresh session and answers the read it was handed
// — the CLI's half of "enqueued and served".
func serveFreshRead(state *State, agent *agentSession) error {
	err := pollUntilWork(state, agent, freshAgentLabel, freshWorkLabel, state.RelayURL)
	if err != nil {
		return err
	}

	item, err := lookupWork(state, freshWorkLabel)
	if err != nil {
		return err
	}

	body, err := replyPayload(state, freshWorkLabel)
	if err != nil {
		return err
	}

	response, err := sendWorkReply(state, item, state.RelayURL, body)
	if err != nil {
		return err
	}

	if response.Status != http.StatusOK {
		return state.fail("replying to the fresh session's read returned %d, want 200: %s",
			response.Status, response.snippet())
	}

	return nil
}
