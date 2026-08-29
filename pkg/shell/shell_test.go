package shell_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// shArgv runs one shell snippet. pkg/shell is the package that owns spawning,
// so a test here reaches for an interpreter the way no caller may.
func shArgv(script string) []string { return []string{"/bin/sh", "-c", script} }

func TestRunCapturesStreamsApart(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(),
		shArgv("echo out; echo err >&2"), shell.Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "out" {
		t.Errorf("stdout: got %q, want %q", result.Stdout, "out")
	}

	if strings.TrimSpace(result.Stderr) != "err" {
		t.Errorf("stderr: got %q, want %q", result.Stderr, "err")
	}
}

// A non-zero exit is an answer, not an error: the three sh() copies this
// replaces disagreed about that, and the disagreement lived in comments.
func TestRunReportsNonZeroExitAsCodeNotError(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(), shArgv("exit 3"), shell.Options{})
	if err != nil {
		t.Fatalf("a non-zero exit must not be an error: %v", err)
	}

	if result.Code != 3 {
		t.Errorf("code: got %d, want 3", result.Code)
	}

	if !errors.Is(result.Err(), shell.ErrExit) {
		t.Errorf("Err(): got %v, want ErrExit", result.Err())
	}
}

func TestRunZeroExitHasNoErr(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(), shArgv("true"), shell.Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.Err() != nil {
		t.Errorf("Err(): got %v, want nil", result.Err())
	}
}

func TestRunMissingBinaryIsNotStarted(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(),
		[]string{"true-bdd-no-such-binary"}, shell.Options{})
	if !errors.Is(err, shell.ErrNotStarted) {
		t.Fatalf("error: got %v, want ErrNotStarted", err)
	}

	if result.Code != shell.NotStarted {
		t.Errorf("code: got %d, want NotStarted", result.Code)
	}
}

func TestRunEmptyArgvIsNotStarted(t *testing.T) {
	t.Parallel()

	_, err := shell.Run(t.Context(), nil, shell.Options{})
	if !errors.Is(err, shell.ErrNotStarted) {
		t.Fatalf("error: got %v, want ErrNotStarted", err)
	}
}

func TestRunTimeout(t *testing.T) {
	t.Parallel()

	_, err := shell.Run(t.Context(), shArgv("sleep 5"),
		shell.Options{Timeout: 50 * time.Millisecond})
	if !errors.Is(err, shell.ErrTimeout) {
		t.Fatalf("error: got %v, want ErrTimeout", err)
	}
}

func TestRunCombinedMergesIntoStdout(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(), shArgv("echo out; echo err >&2"),
		shell.Options{Output: shell.Combined()})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(result.Stdout, "out") || !strings.Contains(result.Stdout, "err") {
		t.Errorf("combined stdout: got %q, want both streams", result.Stdout)
	}

	if result.Stderr != "" {
		t.Errorf("combined stderr: got %q, want empty", result.Stderr)
	}
}

func TestRunStreamsToWriter(t *testing.T) {
	t.Parallel()

	var sink bytes.Buffer

	result, err := shell.Run(t.Context(), shArgv("echo written"),
		shell.Options{Output: shell.To(&sink)})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if strings.TrimSpace(sink.String()) != "written" {
		t.Errorf("writer: got %q, want %q", sink.String(), "written")
	}

	if result.Stdout != "" {
		t.Errorf("Result.Stdout: got %q, want empty when the sink diverts", result.Stdout)
	}
}

func TestRunStdinIsRead(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(), shArgv("cat"),
		shell.Options{Stdin: strings.NewReader("fed")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.Stdout != "fed" {
		t.Errorf("stdout: got %q, want %q", result.Stdout, "fed")
	}
}

func TestRunDirIsHonoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	result, err := shell.Run(t.Context(), shArgv("pwd"), shell.Options{Dir: dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// macOS resolves TempDir through /private, so compare the suffix.
	if !strings.HasSuffix(strings.TrimSpace(result.Stdout), strings.TrimPrefix(dir, "/private")) {
		t.Errorf("pwd: got %q, want %q", result.Stdout, dir)
	}
}

// The distinction this package exists to keep: blanking leaves the key
// present and empty, stripping removes it. Collapsing them changes what a
// nested `claude -p` believes about its own launch.
func TestEnvBlankIsNotStrip(t *testing.T) {
	// No t.Parallel: t.Setenv forbids it.
	t.Setenv("TRUE_BDD_SHELL_PROBE", "set")

	script := `if [ -n "${TRUE_BDD_SHELL_PROBE+x}" ]; then echo present; else echo absent; fi`

	for name, testCase := range map[string]struct {
		env  shell.Env
		want string
	}{
		"blank keeps the key":  {shell.Inherit().Blank("TRUE_BDD_SHELL_PROBE"), "present"},
		"strip removes it":     {shell.Inherit().Strip("TRUE_BDD_SHELL_PROBE"), "absent"},
		"inherit leaves it be": {shell.Inherit(), "present"},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := shell.Run(t.Context(), shArgv(script),
				shell.Options{Env: testCase.env})
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			if strings.TrimSpace(result.Stdout) != testCase.want {
				t.Errorf("got %q, want %q", strings.TrimSpace(result.Stdout), testCase.want)
			}
		})
	}
}

func TestEnvBlankLeavesTheValueEmpty(t *testing.T) {
	// No t.Parallel: t.Setenv forbids it.
	t.Setenv("TRUE_BDD_SHELL_PROBE", "loud")

	result, err := shell.Run(t.Context(), shArgv(`echo "[$TRUE_BDD_SHELL_PROBE]"`),
		shell.Options{Env: shell.Inherit().Blank("TRUE_BDD_SHELL_PROBE")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "[]" {
		t.Errorf("got %q, want %q", strings.TrimSpace(result.Stdout), "[]")
	}
}

func TestEnvSetAppends(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(), shArgv(`echo "$TRUE_BDD_SHELL_ADDED"`),
		shell.Options{Env: shell.Inherit().Set("TRUE_BDD_SHELL_ADDED=yes")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != "yes" {
		t.Errorf("got %q, want %q", strings.TrimSpace(result.Stdout), "yes")
	}
}

func TestResultSignaled(t *testing.T) {
	t.Parallel()

	result, err := shell.Run(t.Context(), shArgv("kill -TERM $$"), shell.Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !result.Signaled() {
		t.Fatalf("Signaled(): got false with code %d, want true", result.Code)
	}

	if result.Signal != syscall.SIGTERM {
		t.Errorf("Signal: got %v, want SIGTERM", result.Signal)
	}
}

func TestFindResolvesAndRefuses(t *testing.T) {
	t.Parallel()

	path, err := shell.Find("sh")
	if err != nil {
		t.Fatalf("find sh: %v", err)
	}

	if path == "" {
		t.Error("find sh: got an empty path")
	}

	_, err = shell.Find("true-bdd-no-such-binary")
	if !errors.Is(err, shell.ErrNotOnPath) {
		t.Errorf("error: got %v, want ErrNotOnPath", err)
	}
}

func TestRequireReportsTheMissingOne(t *testing.T) {
	t.Parallel()

	err := shell.Require("sh")
	if err != nil {
		t.Fatalf("require sh: %v", err)
	}

	err = shell.Require("sh", "true-bdd-no-such-binary")
	if !errors.Is(err, shell.ErrNotOnPath) {
		t.Errorf("error: got %v, want ErrNotOnPath", err)
	}

	if !strings.Contains(err.Error(), "true-bdd-no-such-binary") {
		t.Errorf("error must name the missing binary: %v", err)
	}
}

func TestStartPipesAndWaits(t *testing.T) {
	t.Parallel()

	process, err := shell.Start(t.Context(), shArgv("cat"),
		shell.Options{Output: shell.Pipe()})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	_, err = process.Stdin.Write([]byte("through\n"))
	if err != nil {
		t.Fatalf("write stdin: %v", err)
	}

	err = process.Stdin.Close()
	if err != nil {
		t.Fatalf("close stdin: %v", err)
	}

	echoed, err := readAll(process)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	result, err := process.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}

	if strings.TrimSpace(echoed) != "through" {
		t.Errorf("stdout: got %q, want %q", echoed, "through")
	}

	if result.Code != 0 {
		t.Errorf("code: got %d, want 0", result.Code)
	}
}

func TestStartSignal(t *testing.T) {
	t.Parallel()

	process, err := shell.Start(t.Context(), shArgv("sleep 30"),
		shell.Options{Output: shell.Discard()})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if process.Pid() == 0 {
		t.Error("Pid(): got 0 for a started process")
	}

	err = process.Signal(os.Kill)
	if err != nil {
		t.Fatalf("signal: %v", err)
	}

	result, _ := process.Wait()
	if !result.Signaled() {
		t.Errorf("Signaled(): got false, want true after SIGKILL")
	}
}

func readAll(process *shell.Process) (string, error) {
	var out bytes.Buffer

	_, err := out.ReadFrom(process.Stdout)
	if err != nil {
		return "", err //nolint:wrapcheck // a test helper; the caller names the step.
	}

	return out.String(), nil
}

// A Start whose context is cancelled must not leave the group running.
func TestStartGroupCancelKillsTheGroup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	process, err := shell.Start(ctx, shArgv("sleep 30"),
		shell.Options{Output: shell.Discard(), Group: true, WaitDelay: time.Second})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	cancel()

	_, err = process.Wait()
	if err != nil && !errors.Is(err, shell.ErrNotStarted) {
		t.Logf("wait after cancel returned: %v", err)
	}
}
