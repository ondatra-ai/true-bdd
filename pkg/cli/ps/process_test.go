package ps_test

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
)

func TestMarkedSelectsOnlyEnvironmentIdentity(t *testing.T) {
	const key = "TRUE_BDD_PS_TEST"

	marker := strconv.Itoa(os.Getpid())
	matching := startProcess(t, key+"="+marker, "")
	prefix := startProcess(t, key+"="+marker+"-other", "")
	argument := startProcess(t, key+"=", key+"="+marker)
	otherVariable := startProcess(t, "OTHER_"+key+"="+marker, "")

	processes, err := ps.Marked(t.Context(), key, []string{marker})
	if err != nil {
		t.Fatal(err)
	}

	pids := make([]int, 0, len(processes))
	for _, process := range processes {
		pids = append(pids, process.PID)
	}

	if !slices.Contains(pids, matching) {
		t.Fatalf("marked process %d absent: %v", matching, pids)
	}

	for _, pid := range []int{prefix, argument, otherVariable} {
		if slices.Contains(pids, pid) {
			t.Fatalf("unmarked process %d selected: %v", pid, pids)
		}
	}
}

func startProcess(t *testing.T, entry, argument string) int {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEnvironmentProcessHelper$", "--", argument)

	cmd.Env = append(os.Environ(), "TRUE_BDD_PS_HELPER=yes", entry)

	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	// Start returns after exec, so ps can already read the initial environment.
	return cmd.Process.Pid
}

func TestEnvironmentProcessHelper(t *testing.T) {
	if os.Getenv("TRUE_BDD_PS_HELPER") == "yes" {
		time.Sleep(time.Minute)
	}
}
