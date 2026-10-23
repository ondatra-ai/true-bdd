package drill_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func TestNormalExitCleansDetachedMarkedChild(t *testing.T) {
	runner, record, _ := prepare(t, `#!/bin/sh
exec "$DRILL_TEST_BINARY" -test.run=^TestDrillProcessFixture$
`)
	t.Setenv("DRILL_TEST_BINARY", os.Args[0])
	t.Setenv("DRILL_PROCESS_MODE", "parent")

	realPS, err := exec.LookPath("ps")
	must(t, err)
	t.Setenv("DRILL_REAL_PS", realPS)
	t.Setenv("DRILL_RECORD", filepath.Join(record.Directory, "record.json"))
	fakeBin := t.TempDir()
	put(t, filepath.Join(fakeBin, "ps"), `#!/bin/sh
if ! grep -q '"ended_at": "0001-01-01T00:00:00Z"' "$DRILL_RECORD"; then exit 19; fi
exec "$DRILL_REAL_PS" "$@"
`)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	childFile := filepath.Join(t.TempDir(), "child")
	t.Setenv("DRILL_CHILD_FILE", childFile)
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "detached",
		[]string{buildCommand, codeArgument, fixArgument})
	must(t, err)
	must(t, runner.Supervise(t.Context(), record.Directory, run.Tag))

	child, err := os.ReadFile(childFile)
	must(t, err)
	pid, err := strconv.Atoi(string(child))
	must(t, err)

	t.Setenv("PATH", strings.TrimPrefix(os.Getenv("PATH"), fakeBin+string(os.PathListSeparator)))

	processes, err := ps.Marked(t.Context(), "TRUE_BDD_DRILL_RUN", []string{run.Marker})
	if err != nil || len(processes) != 0 {
		t.Fatalf("marked processes remain: %v %v", processes, err)
	}
	// ps excludes zombies, whose environment is gone and which can no longer act.
	_ = pid
	current, err := drill.Read(record.Directory)
	must(t, err)

	if current.Runs[run.Tag].EndedAt.IsZero() {
		t.Fatal("cleanup not finalized")
	}
}

func TestDrillProcessFixture(t *testing.T) {
	switch os.Getenv("DRILL_PROCESS_MODE") {
	case "parent":
		command := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestDrillProcessFixture$")

		command.Env = append(os.Environ(), "DRILL_PROCESS_MODE=child")
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		must(t, command.Start())
		must(t, os.WriteFile(os.Getenv("DRILL_CHILD_FILE"), []byte(strconv.Itoa(command.Process.Pid)), 0o600))
	case "child":
		time.Sleep(time.Minute)
	}
}

func TestFailedCleanupNeverPublishesEndOrExitCode(t *testing.T) {
	runner, record, _ := prepare(t, "#!/bin/sh\nexit 0\n")
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "bad-ps",
		[]string{buildCommand, codeArgument, fixArgument})
	must(t, err)
	bin := t.TempDir()
	put(t, filepath.Join(bin, "ps"), "#!/bin/sh\nexit 5\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	err = runner.Supervise(t.Context(), record.Directory, run.Tag)
	if err == nil {
		t.Fatal("cleanup failure hidden")
	}

	current, err := drill.Read(record.Directory)
	must(t, err)

	final := current.Runs[run.Tag]
	if final.Status != "cleanup_failed" || final.CleanupError == "" || !final.EndedAt.IsZero() || final.ExitCode != nil {
		t.Fatalf("published end before cleanup: %+v", final)
	}
}

func TestMismatchedCutAndChangedBinaryAreRefused(t *testing.T) {
	runner, record, _ := prepare(t, "#!/bin/sh\nexit 0\n")
	current, err := drill.Read(record.Directory)
	must(t, err)

	cut := current.Cuts["a"]
	cut.Base = "different"
	current.Cuts["a"] = cut
	data, err := json.Marshal(current)
	must(t, err)
	put(t, filepath.Join(record.Directory, "record.json"), string(data))

	_, err = runner.Prepare(t.Context(), record.Directory, "a", "mismatch",
		[]string{buildCommand, codeArgument, fixArgument})
	if err == nil {
		t.Fatal("base mismatch accepted")
	}

	_, err = runner.PrepareOracle(t.Context(), record.Directory, "a")
	if err == nil {
		t.Fatal("oracle base mismatch accepted")
	}

	cut.Base = current.Base
	cut.Digest = "different"
	current.Cuts["a"] = cut
	data, err = json.Marshal(current)
	must(t, err)
	put(t, filepath.Join(record.Directory, "record.json"), string(data))

	_, err = runner.Prepare(t.Context(), record.Directory, "a", "digest",
		[]string{buildCommand, codeArgument, fixArgument})
	if err == nil {
		t.Fatal("binary mismatch accepted")
	}

	current.Base = "different-start"
	data, err = json.Marshal(current)
	must(t, err)
	put(t, filepath.Join(record.Directory, "record.json"), string(data))

	_, err = runner.Cut(t.Context(), record.Directory, "b")
	if err == nil {
		t.Fatal("iteration base mismatch accepted")
	}

	current.Base = record.Base
	data, err = json.Marshal(current)
	must(t, err)
	put(t, filepath.Join(record.Directory, "record.json"), string(data))
	put(t, record.Binary, "changed binary")

	_, err = runner.Cut(t.Context(), record.Directory, "b")
	if err == nil {
		t.Fatal("changed binary accepted")
	}
}
