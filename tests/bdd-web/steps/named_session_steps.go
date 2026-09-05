package steps

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ErrNoNamedRemote is returned when a clause addresses a session by a label
// no Given step started a remote under.
var ErrNoNamedRemote = errors.New("no Given step started a remote under that label")

// ErrNoEarlierRemote is returned when a clause says "in the same project
// tree" and no earlier Given started a remote for it to be the same as.
var ErrNoEarlierRemote = errors.New("no earlier Given started a remote in the project tree")

// startNamedRemote starts a remote in the project tree and files it under
// the label the scenario addresses it by — the Given a scenario about more
// than one session opens with.
func startNamedRemote(state *State, args []string) error {
	label := args[0]

	if state.Tree == nil {
		return state.fail("%w", ErrNoProjectTree)
	}

	remote, err := launchRemote(state, state.Tree.Dir)
	if err != nil {
		return err
	}

	state.Remotes[label] = remote

	return nil
}

// startSiblingRemote is the second-remote Given: the suite materializes ONE
// project tree, so the folder is shared by construction — what this clause
// adds is holding it to being a SECOND remote there.
func startSiblingRemote(state *State, args []string) error {
	if state.Remote == nil && len(state.Remotes) == 0 {
		return state.fail("%w", ErrNoEarlierRemote)
	}

	return startNamedRemote(state, args)
}

// dispatchRunOnNamedSession dispatches on the session the scenario labelled,
// recording the run under its own label. args[0] is the role, discarded —
// held to the product document's role list by the pattern itself.
func dispatchRunOnNamedSession(state *State, args []string) error {
	command, sessionLabel, runLabel := args[1], args[2], args[3]

	session, err := ensureNamedSession(state, sessionLabel)
	if err != nil {
		return err
	}

	return postDispatch(state, session, command, runLabel)
}

// openNamedSessionPage navigates to the page of the labelled session. The
// captured role is discarded, as openSessionPage's is.
func openNamedSessionPage(state *State, args []string) error {
	session, err := ensureNamedSession(state, args[1])
	if err != nil {
		return err
	}

	return showSessionPage(state, session)
}

// ensureNamedSession resolves the labelled remote's registry entry once and
// keeps it, as ensureSession does for the unlabelled one: a clause after a
// disconnect names a session the registry no longer holds.
func ensureNamedSession(state *State, label string) (*sessionSummary, error) {
	session, resolved := state.Sessions[label]
	if resolved {
		return session, nil
	}

	remote, started := state.Remotes[label]
	if !started {
		return nil, state.fail("%w: %q; the scenario started %s",
			ErrNoNamedRemote, label, strings.Join(remoteLabels(state), ", "))
	}

	session, err := awaitSession(state, remote)
	if err != nil {
		return nil, err
	}

	state.Sessions[label] = session

	return session, nil
}

// remoteLabels lists the labels the scenario has started a remote under, so
// a failure names what it has rather than only what it wanted.
func remoteLabels(state *State) []string {
	if len(state.Remotes) == 0 {
		return []string{noneWord}
	}

	labels := make([]string, 0, len(state.Remotes))

	for label := range state.Remotes {
		labels = append(labels, label)
	}

	sort.Strings(labels)

	return labels
}

// stopNamedRemoteWithSignal is the labelled twin of stopRemoteWithSignal, so a
// scenario running two remotes can say which one died. The session is resolved
// BEFORE the signal, as the unlabelled clause does.
func stopNamedRemoteWithSignal(state *State, args []string) error {
	label, name := args[0], args[1]

	remote, started := state.Remotes[label]
	if !started {
		return state.fail("%w: %q; the scenario started %s",
			ErrNoNamedRemote, label, strings.Join(remoteLabels(state), ", "))
	}

	_, err := ensureNamedSession(state, label)
	if err != nil {
		return err
	}

	sig, err := signalNamed(name)
	if err != nil {
		return state.fail("%w", err)
	}

	err = remote.signal(sig)
	if err != nil {
		return state.fail("stopping remote %q (pid %d) with %s: %w",
			label, remote.PID, name, err)
	}

	return nil
}

// assertNamedRemoteRunning holds the labelled remote to still ANSWERING, which
// a registry listing does not say: a session whose remote died reads 504 on the
// status view until the gone timeout drops it (E2E-100 states that split).
func assertNamedRemoteRunning(state *State, args []string) error {
	label := args[0]

	session, err := ensureNamedSession(state, label)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s?view=status", sessionsPath, session.SessionID)
	deadline := time.Now().Add(sessionAppearTimeout)

	var reason string

	for {
		response, readErr := apiGet(state.RelayURL, path)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case response.Status == http.StatusOK:
			state.Response = response

			return nil
		default:
			state.Response = response
			reason = fmt.Sprintf("GET %s returned %d: %s",
				path, response.Status, response.snippet())
		}

		if !time.Now().Before(deadline) {
			return state.fail("remote %q is not answering within %s: %s",
				label, sessionAppearTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}
