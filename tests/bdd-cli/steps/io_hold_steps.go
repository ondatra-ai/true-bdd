package steps

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func registerIOHoldSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the System Architect runs "([^"]+)" `+
		`while a "(exclusive|shared)" hold blocks "([^"]+)" beyond its wait limit$`,
		runWithHoldExpiry)
	suite.Step(`^Git hook variables point outside the project$`, poisonGitEnvironment)
	suite.Step(`^the System Architect runs "([^"]+)" while a "(exclusive|shared)" hold (blocks|permits) "([^"]+)"$`,
		runWithHold)
}

func runWithHold(state *State, args []string) error {
	return runObservedIO(state, args[0], "yes", nil, func(root string, ended <-chan struct{}) error {
		err := awaitIOFile(filepath.Join(root, "tmp/hold-ready"))
		if err != nil {
			return state.fail("launch barrier: %w", err)
		}
		// The directory descriptor spans the engine run; whole-file byte helpers close too early.
		handle, err := os.Open(filepath.Join(root, args[3])) //nolint:forbidigo // hold needs a directory descriptor.
		if err != nil {
			return state.fail("open hold parent: %w", err)
		}
		defer func() { _ = handle.Close() }()

		how := syscall.LOCK_EX
		if args[1] == "shared" {
			how = syscall.LOCK_SH
		}

		err = syscall.Flock(int(handle.Fd()), how|syscall.LOCK_NB)
		if err != nil {
			return state.fail("take hold: %w", err)
		}

		err = disk.Write(filepath.Join(root, "tmp/hold-continue"), []byte("continue\n"), disk.Shared)
		if err != nil {
			return state.fail("release launch barrier: %w", err)
		}

		if args[2] == "expires" {
			return observeHoldExpiry(state, root)
		}

		return verifyHoldObservation(state, root, ended, args[2])
	})
}

func observeHold(state *State, ended <-chan struct{}, effect string) error {
	const (
		blockedWindow = 300 * time.Millisecond
		sharedWindow  = 3 * time.Second
	)

	if effect == "permits" {
		select {
		case <-ended:
			return nil
		case <-time.After(sharedWindow):
			return state.fail("shared read did not complete while another shared hold was held")
		}
	}

	select {
	case <-ended:
		return state.fail("engine returned before the conflicting hold was released")
	case <-time.After(blockedWindow):
		return nil
	}
}

func poisonGitEnvironment(state *State, _ []string) error {
	realGo, err := cli.Find("go")
	if err != nil {
		return state.fail("find genuine Go: %w", err)
	}

	state.Proxy.Env = append(state.Proxy.Env,
		"GIT_DIR=/outside/nonexistent-git", "GIT_INDEX_FILE=/outside/index", "TRUE_BDD_REAL_GO="+realGo)

	return nil
}

func runWithHoldExpiry(state *State, args []string) error {
	return runWithHold(state, []string{args[0], args[1], "expires", args[2]})
}

// Observe the engine's exit before releasing the hold, which also lets the harness take its post-run snapshot.
func observeHoldExpiry(state *State, root string) error {
	pid, err := readIOPID(filepath.Join(root, "tmp/engine.pid"))
	if err != nil {
		return state.fail("hold timeout engine PID: %w", err)
	}

	const (
		latest   = 12 * time.Second
		earliest = 9 * time.Second
		interval = 10 * time.Millisecond
	)

	began := time.Now()
	for time.Since(began) < latest {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			if time.Since(began) < earliest {
				return state.fail("engine returned before the documented hold wait elapsed")
			}

			return nil
		}

		time.Sleep(interval)
	}

	return state.fail("engine did not stop after its documented hold wait")
}

func verifyHoldObservation(state *State, root string, ended <-chan struct{}, effect string) error {
	err := observeHold(state, ended, effect)
	if err != nil || effect == "permits" {
		return err
	}

	pid, err := readIOPID(filepath.Join(root, "tmp/engine.pid"))
	if err != nil {
		return state.fail("held engine PID: %w", err)
	}

	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return state.fail("engine exited before the hold was released")
	}

	return nil
}
