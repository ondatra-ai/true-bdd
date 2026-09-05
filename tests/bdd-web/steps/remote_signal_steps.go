package steps

import (
	"errors"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNotAGroupSignal is returned when a clause names a signal that cannot be
// delivered to a process group.
var ErrNotAGroupSignal = errors.New("not a signal this suite can send to a group")

// remoteExitTimeout is how long the remote has to be gone after the signal: it
// tears its own child down first, which is what the clause after it is about.
const remoteExitTimeout = 60 * time.Second

// registerRemoteSignalSteps binds the interrupt vocabulary: the signal a reader's
// Ctrl-C sends to the whole group, and the exit that must follow it.
func registerRemoteSignalSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the remote's process group is sent "([^"]+)"$`, signalRemoteGroup)
	suite.Step(`^the remote exited$`, assertRemoteExited)
}

// signalRemoteGroup delivers the signal to the GROUP rather than to the leader:
// a clause about what an interrupt does to the descendants cannot be written
// against a signal only the parent received.
func signalRemoteGroup(state *State, args []string) error {
	if state.Remote == nil {
		return state.fail("%w", ErrNoRemote)
	}

	named, err := signalNamed(args[0])
	if err != nil {
		return state.fail("%w", err)
	}

	sig, deliverable := named.(syscall.Signal)
	if !deliverable {
		return state.fail("%w: %q", ErrNotAGroupSignal, args[0])
	}

	// pkg/shell starts the remote in its own process group, so its pid IS the
	// group id a negative target names.
	err = syscall.Kill(-state.Remote.PID, sig)
	if err != nil {
		return state.fail("sending %s to the group of the remote (pid %d): %w",
			args[0], state.Remote.PID, err)
	}

	return nil
}

// assertRemoteExited holds the remote to being gone and reaped, so the clauses
// after it read a relay that has already seen the process go.
func assertRemoteExited(state *State, _ []string) error {
	if state.Remote == nil {
		return state.fail("%w", ErrNoRemote)
	}

	if !state.Remote.waitFor(remoteExitTimeout) {
		return state.fail("the remote (pid %d) was still running %s after the signal, "+
			"want it exited", state.Remote.PID, remoteExitTimeout)
	}

	return nil
}
