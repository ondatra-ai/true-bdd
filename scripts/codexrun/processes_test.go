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

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const failedStatus = "failed"

func TestRunStopsDetachedChildBeforeSummary(t *testing.T) {
	for _, test := range []struct {
		name, mode, status string
		writer             bool
	}{
		{"writer-timeout", "stall", "timed-out", true},
		{"writer-done", "", doneStatus, true},
		{"reader-failed", "fail", failedStatus, false},
		{"reader-done", "", doneStatus, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)
			runner.Limit = 500 * time.Millisecond
			runner.Grace = 20 * time.Millisecond

			if test.writer {
				opts.Name, opts.Kind, opts.Branch, opts.Base = "detached", codeKind, "impl/detached", mainBranch
			}

			installDetachedAgent(t, root)
			t.Setenv("MODE", test.mode)
			t.Setenv("COMMIT", "yes")
			record, err := runner.Start(t.Context(), opts)
			assertStatus(t, record, err, test.status)
			pid := detachedPID(t, root)
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
	path := filepath.Join(root, "bin/codex")
	script := strings.Replace(read(t, path), "echo 'session id: session-175'",
		"echo 'session id: session-175'\n\"$DETACHED_EXECUTABLE\" -test.run=^TestDetachedProcessHelper$", 1)
	write(t, path, script)
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
	for _, writer := range []bool{false, true} {
		t.Run(strconv.FormatBool(writer), func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)

			if writer {
				opts.Name, opts.Kind, opts.Branch, opts.Base = "held", codeKind, "impl/held", mainBranch
			}

			t.Setenv("MODE", "fail")
			record, err := runner.Start(t.Context(), opts)
			assertStatus(t, record, err, failedStatus)
			saved := read(t, filepath.Join(root, "tmp/codex", opts.Name, "run.json"))
			setupDetachedProcess(t, root)

			dir := filepath.Join(record.Workdir, "tmp/child space")

			err = os.MkdirAll(dir, 0o755)
			if err != nil {
				t.Fatal(err)
			}

			cmd := exec.CommandContext(t.Context(), os.Getenv("DETACHED_EXECUTABLE"), "-test.run=^TestDetachedProcessHelper$")

			cmd.Dir = dir

			err = cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			pid := detachedPID(t, root)
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			_, err = runner.Resume(t.Context(), opts.Name, "retry")
			assertHolderRefusal(t, err, pid)

			if got := read(t, filepath.Join(root, "tmp/codex", opts.Name, "run.json")); got != saved {
				t.Fatal("refusal consumed an attempt")
			}

			if writer {
				assertResumeExitCode(t, root, record.Name, pid)
				opts.Name, opts.Kind, opts.Base = "held-fix", fixKind, ""
				_, err = runner.Start(t.Context(), opts)
				assertHolderRefusal(t, err, pid)

				_, err = os.Stat(filepath.Join(root, "tmp/codex", opts.Name))
				if !os.IsNotExist(err) {
					t.Fatal("refused fix created an attempt")
				}
			}

			if syscall.Kill(pid, 0) != nil {
				t.Fatal("refusal killed the holder")
			}
		})
	}
}

func assertResumeExitCode(t *testing.T, root, name string, pid int) {
	t.Helper()

	binary := filepath.Join(root, "bin/codex-runner")

	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../cmd/codex")

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
		"malformed": "printf unexpected",
		"empty":     "exit 0",
	} {
		t.Run(name, func(t *testing.T) {
			root, opts := fixture(t)
			path := filepath.Join(root, "bin/lsof")
			write(t, path, "#!/bin/sh\n"+script+"\n")

			err := os.Chmod(path, 0o755)
			if err != nil {
				t.Fatal(err)
			}

			record, err := codexrun.New(root).Start(t.Context(), opts)
			if err == nil || record.Status != failedStatus || !strings.Contains(record.Reason, "lsof") {
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

	err := os.WriteFile(os.Getenv("DETACHED_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Minute)
	os.Exit(0)
}
