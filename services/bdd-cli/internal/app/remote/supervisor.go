package remote

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SupervisorSubcommand is the hidden CLI verb that runs the resident gated
// group-leader launcher (finding 4). The remote spawns `true-bdd
// <SupervisorSubcommand> <real args...>`.
const SupervisorSubcommand = "remote-supervisor"

const (
	// supervisorReleaseFD is the pipe fd the parent passes as ExtraFiles[0]:
	// the supervisor blocks reading it until the parent has durably recorded the
	// group identity (release), or the pipe reaches EOF (the parent died first).
	supervisorReleaseFD = 3
	// supervisorDrainCap bounds how long the supervisor lingers after the
	// command exits, waiting for the group to drain of other members.
	supervisorDrainCap  = 2 * time.Second
	supervisorDrainPoll = 50 * time.Millisecond
	// supervisorSignalGrace gives a re-raised signal time to land.
	supervisorSignalGrace = 1 * time.Second
	// psGroupFields is the field count of a `ps -o pgid=,pid=` line.
	psGroupFields = 2
)

// RunSupervisor is the RESIDENT GATED group-leader launcher (plan §1.6,
// finding 4): its {pid, start_identity, pgid} is recorded BEFORE release, and
// stays the group's identity even after the command exits — so PGID reuse can never be confused with it.
func RunSupervisor(args []string) int {
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	if !awaitRelease() {
		// The parent died before recording our identity — never mutate.
		return 0
	}

	self, err := os.Executable()
	if err != nil {
		return 1
	}

	//nolint:gosec // self is our own binary; args are the already-validated command vector
	cmd := exec.CommandContext(context.Background(), self, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// No Setpgid: the command inherits the supervisor's group, so the supervisor
	// stays the verifiable group leader.

	startErr := cmd.Start()
	if startErr != nil {
		return 1
	}

	waitErr := cmd.Wait()

	waitGroupDrain()

	return propagateExit(waitErr)
}

// awaitRelease blocks until the parent writes the release byte (proceed) or the
// pipe reaches EOF before any byte (the parent died before recording our
// identity → fail closed).
func awaitRelease() bool {
	pipe := os.NewFile(supervisorReleaseFD, "release")
	if pipe == nil {
		return false
	}
	defer func() { _ = pipe.Close() }()

	buf := make([]byte, 1)
	n, _ := pipe.Read(buf)

	return n == 1
}

// waitGroupDrain lingers (bounded) until no other process remains in the
// supervisor's group, so the group leader outlives any command grandchild.
func waitGroupDrain() {
	pgid := os.Getpid() // the supervisor is the group leader: pgid == its pid
	deadline := time.Now().Add(supervisorDrainCap)

	for time.Now().Before(deadline) {
		if !groupHasOtherMembers(pgid) {
			return
		}

		time.Sleep(supervisorDrainPoll)
	}
}

// groupHasOtherMembers reports whether any process OTHER than the supervisor is
// still in its process group. An unreadable process table ⇒ do not linger.
func groupHasOtherMembers(pgid int) bool {
	out, err := exec.CommandContext(context.Background(), "ps", "-A", "-o", "pgid=,pid=").Output()
	if err != nil {
		return false
	}

	self := os.Getpid()

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != psGroupFields {
			continue
		}

		group, gErr := strconv.Atoi(fields[0])
		pid, pErr := strconv.Atoi(fields[1])

		if gErr != nil || pErr != nil {
			continue
		}

		if group == pgid && pid != self {
			return true
		}
	}

	return false
}

// propagateExit re-raises the command's terminating signal on the supervisor
// (so the parent's Wait sees a faithful WaitStatus) or exits with its code.
func propagateExit(waitErr error) int {
	if waitErr == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				sig := status.Signal()
				signal.Reset(sig)
				_ = syscall.Kill(os.Getpid(), sig)

				time.Sleep(supervisorSignalGrace)
			}

			return status.ExitStatus()
		}

		return exitErr.ExitCode()
	}

	return 1
}
