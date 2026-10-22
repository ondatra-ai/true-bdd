package shell

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// StopProcesses sends SIGTERM to individual processes, then SIGKILL after grace.
// Already exited processes are harmless; permission failures are returned.
func StopProcesses(pids []int, grace time.Duration) error {
	var problems []error

	for _, pid := range pids {
		if pid <= 1 || pid == os.Getpid() {
			return fmt.Errorf("%w: refusing to stop PID %d", syscall.EINVAL, pid)
		}
	}

	for _, pid := range pids {
		err := signalProcess(pid, syscall.SIGTERM)
		if err != nil {
			problems = append(problems, err)
		}
	}

	if waitProcesses(pids, grace) {
		return errors.Join(problems...)
	}

	for _, pid := range pids {
		err := signalProcess(pid, syscall.SIGKILL)
		if err != nil {
			problems = append(problems, err)
		}
	}

	return errors.Join(problems...)
}

func waitProcesses(pids []int, grace time.Duration) bool {
	const interval = 10 * time.Millisecond

	for end := time.Now().Add(grace); time.Now().Before(end); {
		remaining := false

		for _, pid := range pids {
			if !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
				remaining = true
			}
		}

		if !remaining {
			return true
		}

		time.Sleep(interval)
	}

	return false
}

func signalProcess(pid int, sig syscall.Signal) error {
	err := syscall.Kill(pid, sig)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("sending %s to PID %d: %w", sig, pid, err)
	}

	return nil
}
