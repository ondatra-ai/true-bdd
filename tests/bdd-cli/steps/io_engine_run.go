package steps

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

const ioLaunchScript = `#!/bin/sh
set -eu
mkdir -p tmp
echo $$ > tmp/engine.pid
if [ "$TRUE_BDD_IO_BARRIER" = yes ]; then
  printf ready > tmp/hold-ready
  while [ ! -f tmp/hold-continue ]; do sleep 0.01; done
fi
exec "$TRUE_BDD_IO_ENGINE" "$@"
`

// Launch after the harness snapshot, so an observer exercises the engine's IO rather than the harness's.
func runObservedIO(
	state *State, command, barrier string, env []string, observe func(string, <-chan struct{}) error,
) error {
	original := state.Harness
	wrapper := filepath.Join(original.SessionRoot, state.Fixture.Name+"-engine")

	err := disk.Write(wrapper, []byte(ioLaunchScript), disk.Private)
	if err != nil {
		return state.fail("write IO launcher: %w", err)
	}

	err = disk.Executable(wrapper)
	if err != nil {
		return state.fail("make IO launcher executable: %w", err)
	}

	harness := *original
	harness.BinPath = wrapper

	state.Harness = &harness
	defer func() { state.Harness = original }()

	env = append(env, "TRUE_BDD_IO_ENGINE="+original.BinPath, "TRUE_BDD_IO_BARRIER="+barrier)
	root := runner.RunDir(original.SessionRoot, state.Fixture.Name)
	ended := make(chan struct{})

	var runErr error
	go func() {
		runErr = runEngine(state, command, env...)

		close(ended)
	}()

	observationErr := observe(root, ended)
	if observationErr != nil {
		killIOEngine(root)
	}

	const limit = 5 * time.Second
	select {
	case <-ended:
	case <-time.After(limit):
		killIOEngine(root)
		<-ended

		return state.fail("IO engine did not finish after observation")
	}

	if observationErr != nil {
		return observationErr
	}

	return runErr
}

func killIOEngine(root string) {
	pid, err := readIOPID(filepath.Join(root, "tmp/engine.pid"))
	if err == nil && pid > 1 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func awaitIOFile(path string) error {
	const (
		limit    = 10 * time.Second
		interval = 10 * time.Millisecond
	)

	for end := time.Now().Add(limit); time.Now().Before(end); {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return nil
		}

		time.Sleep(interval)
	}

	return os.ErrDeadlineExceeded
}

func registerIOSteps(suite *bddgo.Suite[State]) {
	registerIOHoldSteps(suite)
	registerIOClaudeSteps(suite)
	registerIOAtomicSteps(suite)
	registerIOCancelSteps(suite)
}
