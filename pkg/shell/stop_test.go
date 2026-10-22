package shell_test

import (
	"bufio"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

func TestStopProcesses(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, script string
		killed       bool
	}{
		{"graceful", `trap 'exit 0' TERM; echo ready; while :; do sleep 0.01; done`, false},
		{"ignores-term", `trap '' TERM; echo ready; exec sleep 10`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cmd, ended := stoppableChild(t, test.script)

			err := shell.StopProcesses([]int{cmd.Process.Pid}, 30*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}

			select {
			case err := <-ended:
				if !test.killed && err != nil {
					t.Fatalf("denied graceful exit: %v", err)
				}

				if test.killed {
					status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
					if !ok || status.Signal() != syscall.SIGKILL {
						t.Fatalf("expected SIGKILL: %v", err)
					}
				}
			case <-time.After(time.Second):
				t.Fatal("process survived shutdown")
			}
		})
	}
}

func stoppableChild(t *testing.T, script string) (*exec.Cmd, <-chan error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", "-c", script)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = cmd.Process.Kill() })

	if !bufio.NewScanner(stdout).Scan() {
		t.Fatal("child did not become ready")
	}

	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()

	return cmd, ended
}
