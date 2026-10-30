package steps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

func registerIOCancelSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the System Architect cancels "([^"]+)" after a CLI descendant starts$`, cancelActiveCLI)
	suite.Step(`^the CLI descendant has stopped$`, assertDescendantStopped)
}

//truebdd:concept when "a command runs under interference"
func cancelActiveCLI(state *State, args []string) error {
	fixture, err := filepath.Abs(state.Fixture.Dir)
	if err != nil {
		return state.fail("fixture: %w", err)
	}

	env := []string{"PATH=" + filepath.Join(fixture, "cli") + string(os.PathListSeparator) + os.Getenv("PATH")}

	return runObservedIO(state, args[0], "no", env, func(root string, _ <-chan struct{}) error {
		path := filepath.Join(root, "tmp/descendant.pid")

		err := awaitIOFile(path)
		if err != nil {
			return state.fail("CLI did not start its descendant: %w", err)
		}

		pid, err := readIOPID(path)
		if err != nil {
			return state.fail("read descendant: %w", err)
		}

		state.T.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

		engine, err := readIOPID(filepath.Join(root, "tmp/engine.pid"))
		if err != nil {
			return state.fail("read engine PID: %w", err)
		}

		err = syscall.Kill(engine, syscall.SIGINT)
		if err != nil {
			return state.fail("interrupt engine: %w", err)
		}

		return nil
	})
}

func readIOPID(path string) (int, error) {
	data, err := disk.Read(path)
	if err != nil {
		return 0, fmt.Errorf("read PID: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("decode PID: %w", err)
	}

	return pid, nil
}

//truebdd:concept then "AI CLI observations"
func assertDescendantStopped(state *State, _ []string) error {
	pid, err := readIOPID(filepath.Join(state.Result.TmpDir, "tmp/descendant.pid"))
	if err != nil {
		return state.fail("read descendant PID: %w", err)
	}

	const (
		limit    = 3 * time.Second
		interval = 10 * time.Millisecond
	)

	for end := time.Now().Add(limit); time.Now().Before(end); {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return nil
		}

		time.Sleep(interval)
	}

	return state.fail("CLI descendant %d survived engine cancellation", pid)
}
