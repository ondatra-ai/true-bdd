package drill_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gitcli "github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func TestDetachedCommandSurvivesLauncherAndStopsCleanly(t *testing.T) {
	realGo, err := exec.LookPath("go")
	must(t, err)
	worker := filepath.Join(t.TempDir(), "drill")
	build := exec.CommandContext(t.Context(), realGo, buildCommand, "-trimpath", "-o", worker, "../cmd/drill")
	build.Env = gitcli.ScrubEnvironment(os.Environ())

	output, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("building test worker: %s: %v", output, err)
	}

	runner, record, _ := prepare(t, `#!/bin/sh
printf '\nCLARIFICATION NEEDED\n[1/1] What is specified?\n1) Follow Specification\n0) Other\n\n    Your answer: '
read answer
`)
	command := exec.CommandContext(t.Context(),
		worker,
		"run",
		"--iteration",
		record.Directory,
		"a",
		"detached-worker",
		"--",
		buildCommand,
		codeArgument,
		fixArgument)

	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("launch: %s: %v", output, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = runner.Stop(ctx, record.Directory, "detached-worker")
	})

	final := waitRun(t,
		record.Directory,
		"detached-worker",
		func(run drill.Attempt) bool { return run.Status == pausedStatus })
	if final.WorkerPID == 0 {
		t.Fatal("worker PID not recorded")
	}

	workers, err := ps.Marked(t.Context(), "TRUE_BDD_DRILL_WORKER", []string{final.Marker})
	must(t, err)

	if len(workers) != 1 {
		t.Fatalf("worker disappeared with caller: %v", workers)
	}

	must(t, runner.Stop(t.Context(), record.Directory, final.Tag))

	finished := waitRun(t, record.Directory, final.Tag, func(run drill.Attempt) bool { return !run.EndedAt.IsZero() })
	if finished.ExitCode == nil || finished.CleanupError != "" {
		t.Fatalf("stop did not finalize cleanup: %+v", finished)
	}

	waitNoWorkers(t, final.Marker)
}

func waitRun(t *testing.T, directory, tag string, ready func(drill.Attempt) bool) drill.Attempt {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		record, err := drill.Read(directory)
		must(t, err)

		if run := record.Runs[tag]; ready(run) {
			return run
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("run did not reach expected state")

	return drill.Attempt{}
}

func waitNoWorkers(t *testing.T, marker string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		workers, err := ps.Marked(t.Context(), "TRUE_BDD_DRILL_WORKER", []string{marker})
		must(t, err)

		if len(workers) == 0 {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("worker remains")
}

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}
