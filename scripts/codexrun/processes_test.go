package codexrun_test

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	gitcli "github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const failedStatus = "failed"

func TestRunStopsDetachedChildBeforeSummary(t *testing.T) {
	for _, test := range []struct {
		name, mode, status string
		resume             bool
	}{
		{name: "writer-timeout", mode: "stall", status: "timed-out"},
		{name: "writer-done", status: doneStatus},
		{name: "writer-failed", mode: "fail", status: failedStatus},
		{name: "writer-unavailable", mode: limitStatus, status: unavailableStatus},
		{name: "writer-resume", status: doneStatus, resume: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)
			runner.Limit = 500 * time.Millisecond
			runner.Grace = 20 * time.Millisecond

			opts.Name, opts.Kind, opts.Branch, opts.Base = "detached", codeKind, "impl/detached", mainBranch

			if test.resume {
				t.Setenv("MODE", "fail")
				record, err := runner.Start(t.Context(), opts)
				assertStatus(t, record, err, failedStatus)
			}

			installDetachedAgent(t, root)
			t.Setenv("MODE", test.mode)
			t.Setenv("COMMIT", "yes")

			var (
				record codexrun.Record
				err    error
			)
			if test.resume {
				record, err = runner.Resume(t.Context(), opts.Name, "retry")
			} else {
				record, err = runner.Start(t.Context(), opts)
			}

			assertStatus(t, record, err, test.status)
			pid := detachedPID(t, root)

			marker := read(t, filepath.Join(root, "capture.pid.marker"))
			if !strings.HasPrefix(marker, opts.Name+":"+strconv.Itoa(record.Attempt)+":") {
				t.Fatalf("child did not inherit this attempt's marker: %q", marker)
			}

			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

			if syscall.Kill(pid, 0) == nil {
				t.Fatalf("summary published while detached child %d is alive: %s", pid, record.Summary())
			}
		})
	}
}

func installDetachedAgent(t *testing.T, root string) {
	t.Helper()
	setupDetachedProcess(t, root)
	t.Setenv("DETACHED_LEAVE", "yes")

	for _, agent := range []string{codexCommand} {
		path := filepath.Join(root, "bin", agent)
		script := strings.Replace(read(t, path), "cat > \"$CAPTURE.stdin\"",
			"cat > \"$CAPTURE.stdin\"\n\"$DETACHED_EXECUTABLE\" -test.run=^TestDetachedProcessHelper$", 1)
		write(t, path, script)
	}
}

func setupDetachedProcess(t *testing.T, root string) {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DETACHED_EXECUTABLE", executable)
	t.Setenv("DETACHED_MODE", "spawn")
	t.Setenv("DETACHED_PID", filepath.Join(root, "capture.pid"))
}

func TestResumeAndFixRefuseDetachedHolders(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Name, opts.Kind, opts.Branch, opts.Base = "held", codeKind, "impl/held", mainBranch

	t.Setenv("MODE", "fail")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, failedStatus)
	saved := read(t, filepath.Join(root, "tmp/codex", opts.Name, "run.json"))
	// Ownership follows the environment even after a child leaves the worktree.
	pid := startHolder(t, root, root, strings.TrimSpace(read(t, filepath.Join(root, "capture.marker"))))
	_, err = runner.Resume(t.Context(), opts.Name, "retry")
	assertHolderRefusal(t, err, pid)

	if got := read(t, filepath.Join(root, "tmp/codex", opts.Name, "run.json")); got != saved {
		t.Fatal("refusal consumed an attempt")
	}

	assertResumeExitCode(t, root, record.Name, pid)
	opts.Name, opts.Kind, opts.Base = "held-fix", fixKind, ""
	_, err = runner.Start(t.Context(), opts)
	assertHolderRefusal(t, err, pid)

	_, err = os.Stat(filepath.Join(root, "tmp/codex", opts.Name))
	if !os.IsNotExist(err) {
		t.Fatal("refused fix created an attempt")
	}

	if syscall.Kill(pid, 0) != nil {
		t.Fatal("refusal killed the holder")
	}
}

func TestWriterLeavesUnmarkedWorktreeProcess(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Name, opts.Kind, opts.Branch, opts.Base = "unmarked", codeKind, "impl/unmarked", mainBranch

	t.Setenv("MODE", "fail")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, failedStatus)
	pid := startHolder(t, root, filepath.Join(record.Worktree, "tmp/child space"), "")
	t.Setenv("MODE", "")
	t.Setenv("COMMIT", "yes")
	record, err = runner.Resume(t.Context(), opts.Name, "retry")
	assertStatus(t, record, err, doneStatus)

	if syscall.Kill(pid, 0) != nil {
		t.Fatal("writer stopped an unmarked worktree process")
	}

	opts.Name, opts.Kind, opts.Base = "unmarked-fix", fixKind, ""
	record, err = runner.Start(t.Context(), opts)
	assertStatus(t, record, err, doneStatus)

	if syscall.Kill(pid, 0) != nil {
		t.Fatal("fix stopped an unmarked worktree process")
	}
}

func TestReaderLeavesCheckoutAndOtherRunProcesses(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	rootPID := startHolder(t, root, root, "")
	otherPID := startHolder(t, root, filepath.Join(root, "tmp/codex-worktrees/other"), "other:1")
	// Readers must not depend on discovery at end or resume.
	write(t, filepath.Join(root, "bin/lsof"), "#!/bin/sh\nexit 1\n")
	write(t, filepath.Join(root, "bin/ps"), "#!/bin/sh\nexit 1\n")

	for _, name := range []string{"lsof", "ps"} {
		err := os.Chmod(filepath.Join(root, "bin", name), 0o755)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("MODE", "fail")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, failedStatus)
	t.Setenv("MODE", "")
	record, err = runner.Resume(t.Context(), opts.Name, "retry")
	assertStatus(t, record, err, doneStatus)

	for _, pid := range []int{rootPID, otherPID} {
		if syscall.Kill(pid, 0) != nil {
			t.Fatalf("reader stopped unrelated process %d", pid)
		}
	}
}

func startHolder(t *testing.T, root, dir, marker string) int {
	t.Helper()
	setupDetachedProcess(t, root)

	err := os.Remove(os.Getenv("DETACHED_PID"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(t.Context(), os.Getenv("DETACHED_EXECUTABLE"), "-test.run=^TestDetachedProcessHelper$")
	cmd.Dir = dir

	cmd.Env = append(os.Environ(), "TRUE_BDD_CODEX_RUN="+marker)

	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}

	pid := detachedPID(t, root)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	return pid
}

func assertResumeExitCode(t *testing.T, root, name string, pid int) {
	t.Helper()

	binary := filepath.Join(root, "bin/codex-runner")

	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../cmd/codex")
	build.Env = gitcli.ScrubEnvironment(os.Environ())

	output, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build runner: %v: %s", err, output)
	}

	cmd := exec.CommandContext(t.Context(), binary, "resume", "--name", name, "--note", "retry")
	cmd.Dir = root
	output, err = cmd.CombinedOutput()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(output), strconv.Itoa(pid)) {
		t.Fatalf("expected exit 2 naming PID %d: %v: %s", pid, err, output)
	}
}

func assertHolderRefusal(t *testing.T, err error, pid int) {
	t.Helper()

	if !errors.Is(err, codexrun.ErrRefused) || !strings.Contains(err.Error(), strconv.Itoa(pid)) ||
		!strings.Contains(err.Error(), ".test") {
		t.Fatalf("expected refusal naming holder %d, got %v", pid, err)
	}
}

func TestDiscoveryFailurePreventsCompletion(t *testing.T) {
	for name, script := range map[string]string{
		failedStatus: "echo discovery-failed >&2; exit 1",
		"malformed":  "printf unexpected",
		"empty":      "exit 0",
	} {
		t.Run(name, func(t *testing.T) {
			root, opts := fixture(t)
			opts.Name, opts.Kind, opts.Branch, opts.Base = "discovery", codeKind, "impl/discovery", mainBranch
			path := filepath.Join(root, "bin/ps")
			write(t, path, "#!/bin/sh\n"+script+"\n")

			err := os.Chmod(path, 0o755)
			if err != nil {
				t.Fatal(err)
			}

			record, err := codexrun.New(root).Start(t.Context(), opts)
			if err == nil || record.Status != failedStatus || !strings.Contains(record.Reason, "ps") {
				t.Fatalf("reported completion without discovering holders: %+v %v", record, err)
			}
		})
	}
}

func detachedPID(t *testing.T, root string) int {
	t.Helper()

	pid, err := strconv.Atoi(strings.TrimSpace(read(t, filepath.Join(root, "capture.pid"))))
	if err != nil {
		t.Fatal(err)
	}

	return pid
}

func TestDetachedProcessHelper(t *testing.T) {
	mode := os.Getenv("DETACHED_MODE")
	if mode == "" {
		return
	}

	if mode == "spawn" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestDetachedProcessHelper$")

		cmd.Env = append(os.Environ(), "DETACHED_MODE=child")

		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		err := cmd.Start()
		if err != nil {
			t.Fatal(err)
		}

		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			_, err := os.Stat(os.Getenv("DETACHED_PID"))
			if err == nil {
				os.Exit(0)
			}

			time.Sleep(time.Millisecond)
		}

		t.Fatal("detached child did not become ready")
	}

	signal.Ignore(syscall.SIGTERM)

	if os.Getenv("DETACHED_LEAVE") == "yes" {
		t.Chdir("/")
	}

	marker := []byte(os.Getenv("TRUE_BDD_CODEX_RUN"))

	err := os.WriteFile(os.Getenv("DETACHED_PID")+".marker", marker, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(os.Getenv("DETACHED_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Minute)
	os.Exit(0)
}
