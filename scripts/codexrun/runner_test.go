package codexrun_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitcli "github.com/ondatra-ai/true-bdd/pkg/cli/git"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const (
	mainBranch    = "main"
	runningStatus = "running"
	fixKind       = "fix"
	codeKind      = "code"
	doneStatus    = "done"
)

func fixture(t *testing.T) (string, codexrun.Options) {
	t.Helper()
	root := t.TempDir()

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CODEX_TEST_GIT", realGit)

	bin := filepath.Join(root, "bin")

	err = os.Mkdir(bin, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	for name, script := range map[string]string{
		"git": `#!/bin/sh
printf '%s\n' "$@" >> "$CAPTURE.git"
if [ "$GIT_FAIL_ADD" = yes ] && [ "$1" = worktree ] && [ "$2" = add ]; then exit 1; fi
for name in $("$CODEX_TEST_GIT" rev-parse --local-env-vars); do unset "$name"; done
exec "$CODEX_TEST_GIT" "$@"
`,
		"codex": `#!/bin/sh
[ "$1" = --fixture-ready ] && exit 0
if [ "$1" = login ]; then
 case "$LOGIN" in
 silent) sleep 5;;
 broken) exit 1;;
 out) echo 'Not logged in'; exit 1;;
 *) echo 'Logged in using ChatGPT';;
 esac
 exit 0
fi
printf '%s\n' "$TRUE_BDD_CODEX_RUN" > "$CAPTURE.marker"
printf '%s\n' "$@" > "$CAPTURE.args"
cat > "$CAPTURE.stdin"
echo 'session id: session-175'
while [ $# -gt 0 ]; do
 case "$1" in -C) shift; folder="$1";; -o) shift; report="$1";; esac
 shift
done
if [ "$PARTIAL" = yes ]; then printf 'partial edit' > "$folder/partial.go"; fi
case "$MODE" in
 expired) echo 'ERROR: Your access token could not be refreshed.'; exit 1;;
 stall) sleep 10;;
 limit) echo "ERROR: You've hit your usage limit."; exit 1;;
 spend-cap) echo 'ERROR: You hit your spend cap set in your workspace. Increase your spend cap to continue.'; exit 1;;
 fail) exit 1;;
esac
if [ "$COMMIT" = yes ]; then
 git -C "$folder" -c core.hooksPath=/dev/null commit --allow-empty -m 'test commit' >/dev/null
fi
printf 'finished report' > "$report"
`,
		"gh": `#!/bin/sh
printf '{"number":175,"title":"Delegation","body":"saved issue body","comments":[{"body":"saved comment"}]}'
`,
	} {
		err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "home"))
	t.Setenv("CAPTURE", filepath.Join(root, "capture"))

	prepareFakeAgents(t, bin)

	seedFixture(t, root)
	brief := filepath.Join(root, "brief.md")
	write(t, brief, "Review issue #175")

	return root, codexrun.Options{Name: "review-175", Kind: "review", Brief: brief}
}

func seedFixture(t *testing.T, root string) {
	t.Helper()

	for _, args := range [][]string{
		{"init", "-b", mainBranch}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"},
	} {
		git(t, root, args...)
	}

	write(t, filepath.Join(root, ".gitignore"), "tmp/\nbin/\ncapture*\nhome/\nbrief.md\n")
	git(t, root, "add", ".gitignore")
	git(t, root, "commit", "-m", "seed")
}

// Prepare executables before watchdog timing: first execution of a new
// script can exceed 200ms on macOS (observed during #183's gate run).
func prepareFakeAgents(t *testing.T, bin string) {
	t.Helper()

	for _, name := range []string{codexCommand} {
		cmd := exec.CommandContext(t.Context(), filepath.Join(bin, name), "--fixture-ready")

		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("prepare fake %s: %s %v", name, data, err)
		}
	}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	cmd.Env = gitcli.ScrubEnvironment(os.Environ())

	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, data, err)
	}

	return strings.TrimSpace(string(data))
}

func write(t *testing.T, path, body string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte(body), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestReadersAndResume(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Issues = []string{"175"}

	t.Setenv("MODE", "fail")

	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, "failed")

	if record.SessionID != "session-175" {
		t.Fatalf("start: %+v %v", record, err)
	}

	if !strings.Contains(read(t, filepath.Join(root, "capture.args")), "read-only") {
		t.Fatal("reader not sandboxed")
	}

	if !strings.Contains(read(t, filepath.Join(root, "tmp/codex/review-175/issues/175.md")), "saved comment") {
		t.Fatal("missing issue comments")
	}

	t.Setenv("MODE", "")

	record, err = runner.Resume(t.Context(), opts.Name, "explain the failure")
	assertStatus(t, record, err, doneStatus)

	if record.Attempt != 2 {
		t.Fatalf("resume: %+v %v", record, err)
	}

	if !strings.Contains(read(t, filepath.Join(root, "capture.args")), "resume\nsession-175") {
		t.Fatal("wrong session")
	}

	_, err = runner.Resume(t.Context(), opts.Name, "again")
	if err == nil {
		t.Fatal("third attempt accepted")
	}
}

func TestWritersAndFix(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Name = "code-175"
	opts.Kind = codeKind
	opts.Branch = "impl/175"
	opts.Base = mainBranch

	t.Setenv("COMMIT", "yes")

	record, err := runner.Start(t.Context(), opts)
	if err != nil || record.Status != doneStatus {
		t.Fatalf("code: %+v %v", record, err)
	}

	if git(t, record.Worktree, "branch", "--show-current") != opts.Branch {
		t.Fatal("wrong branch")
	}

	if !strings.Contains(read(t, filepath.Join(root, "capture.args")), "danger-full-access") {
		t.Fatal("writer sandbox")
	}

	opts.Name = "fix-175"
	opts.Kind = fixKind
	opts.Base = ""

	fixed, err := runner.Start(t.Context(), opts)
	assertStatus(t, fixed, err, doneStatus)

	if fixed.Worktree != record.Worktree || fixed.Since == record.Since {
		t.Fatalf("fix: %+v %v", fixed, err)
	}

	opts.Name = "fix-empty"

	t.Setenv("COMMIT", "")

	empty, err := runner.Start(t.Context(), opts)
	if err != nil || empty.Status != "incomplete" {
		t.Fatalf("no commit: %+v %v", empty, err)
	}
}

func TestUnavailableReaders(t *testing.T) {
	for _, mode := range []string{"out", limitStatus, spendCapStatus, "silent"} {
		t.Run(mode, func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)
			runner.Stall = 500 * time.Millisecond
			runner.Grace = 10 * time.Millisecond

			if mode == limitStatus || mode == spendCapStatus {
				t.Setenv("MODE", mode)
			} else {
				t.Setenv("LOGIN", mode)
			}

			record, err := runner.Start(t.Context(), opts)
			if err != nil || record.Status != unavailableStatus {
				t.Fatalf("reader: %+v %v", record, err)
			}
		})
	}
}

func TestWatchdogAndWeekly(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	runner.Stall = 500 * time.Millisecond
	runner.Grace = 10 * time.Millisecond

	t.Setenv("MODE", "stall")

	event := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339), "type": "event_msg",
		"payload": map[string]any{"type": "token_count", "rate_limits": map[string]any{
			"secondary": map[string]any{
				"window_minutes": 10080, "used_percent": 93, "resets_at": time.Now().Add(time.Hour).Unix(),
			},
		}},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, "home/sessions/rollout-test.jsonl"), string(data)+"\n{unfinished")

	record, err := runner.Start(t.Context(), opts)
	if err != nil || record.Status != "stalled" || record.WeeklyLimit == nil || record.WeeklyLimit.UsedPercent != 93 {
		t.Fatalf("watch/weekly: %+v %v", record, err)
	}
}

func assertStatus(t *testing.T, record codexrun.Record, err error, status string) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}

	if record.Status != status {
		t.Fatalf("status %s, want %s: %+v", record.Status, status, record)
	}
}
