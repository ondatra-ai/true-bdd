package commit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/mandate"
)

const (
	gitBin = "git"
	ghBin  = "gh"

	// The trunk, and the base a narrowed gate selection diffs against.
	trunk    = "main"
	altTrunk = "master"

	dirMode  = 0o755
	fileMode = 0o600

	// StateDir holds what a run produces, so a message can be read after it.
	StateDir = "tmp/commit"

	diagnosticLimit = 800
)

// Run is one invocation, from one checkout. Every field is settled by Start.
type Run struct {
	// unattended reports that task-handle stamped a mandate: nobody is at the
	// terminal, so the gates narrow to the ones this diff needs.
	unattended bool
}

// Start answers where we are and whether this can be committed at all — every
// precondition checked once, up front, so Main is only the sequence.
func Start(args []string) *Run {
	if len(args) > 0 {
		usage("usage: commit — no arguments. Everything comes from the checkout.")
	}

	top, err := exec.CommandContext(context.Background(),
		gitBin, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		usage("not inside a git repository")
	}

	err = os.Chdir(strings.TrimSpace(string(top)))
	if err != nil {
		usage("cannot enter the repository root: " + err.Error())
	}

	for _, tool := range []string{gitBin, ghBin, "claude"} {
		_, err := exec.LookPath(tool)
		if err != nil {
			usage(tool + " not found in PATH")
		}
	}

	return &Run{unattended: mandate.Active(".")}
}

// Main is the whole sequence. Any step that cannot finish stops the run.
func (r *Run) Main() {
	r.gates()
	r.scanRecordings()
	r.syncDocUniverse()
	r.updateMemory()
	r.stage()
	r.ensureBranch()
	r.commit()

	r.banner("pull request: " + r.UpdatePR())
}

// gates runs the quality pipeline. Under a mandate it narrows to the gates the
// diff needs, which is ~2s on a documentation ticket against ~140s; selection
// is LOCAL ONLY, so CI still catches whatever the selector skipped.
func (r *Run) gates() {
	r.banner("gates")

	selected := gates.All

	if r.unattended {
		changed, err := gates.Changed(r.trunkRef())
		if err != nil {
			r.dief("selecting gates: %v", err)
		}

		selected = gates.Select(changed)
		r.logf("%d/%d gates for %d changed path(s) vs %s",
			len(selected), len(gates.All), len(changed), r.trunkRef())
	}

	err := gates.Run(os.Stdout, selected)
	if err != nil {
		r.dief("the gates are red — nothing was committed:\n  %v", err)
	}
}

// trunkRef is main, or master where that is what the repository calls it.
func (r *Run) trunkRef() string {
	if r.git("show-ref", "--verify", "--quiet", "refs/remotes/origin/"+trunk).code == 0 {
		return trunk
	}

	if r.git("show-ref", "--verify", "--quiet", "refs/remotes/origin/"+altTrunk).code == 0 {
		return altTrunk
	}

	return trunk
}

// ------------------------------------------------------------------ output

func (r *Run) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, "  "+format+"\n", args...)
}

func (r *Run) banner(message string) {
	_, _ = fmt.Fprintf(os.Stdout, "\n══ %s ══\n", message)
}

// dief stops the run with a diagnosis. Nothing here is swallowed.
func (r *Run) dief(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\nSTOPPED: "+format+"\n", args...)
	os.Exit(1)
}

// usage stops before the run has a context to report against.
func usage(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

// ------------------------------------------------------------------- shell

type result struct {
	stdout string
	stderr string
	code   int
}

// sh runs an argv list. No shell, ever.
func (r *Run) sh(argv []string, stream bool) result {
	//nolint:gosec // every argv in this package is a literal or a parsed value.
	command := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	// Blanked, not removed: a child should know it is not interactive. Only a
	// nested `claude -p` needs the variable gone — see claudecli.
	command.Env = append(os.Environ(), "CLAUDECODE=")

	var stdout, stderr bytes.Buffer

	if stream {
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
	} else {
		command.Stdout, command.Stderr = &stdout, &stderr
	}

	err := command.Run()

	return result{stdout: stdout.String(), stderr: stderr.String(), code: exitCode(err)}
}

func (r *Run) git(args ...string) result {
	return r.sh(append([]string{gitBin}, args...), false)
}

// gitChecked runs a git command and stops the run if it fails.
func (r *Run) gitChecked(args ...string) string {
	out := r.git(args...)
	if out.code != 0 {
		r.dief("git %s failed (%d):\n%s", strings.Join(args, " "), out.code,
			textutil.Truncate(firstNonEmpty(out.stderr, out.stdout), diagnosticLimit))
	}

	return out.stdout
}

// gh runs a gh command and stops the run if it fails.
func (r *Run) gh(args ...string) string {
	out := r.sh(append([]string{ghBin}, args...), false)
	if out.code != 0 {
		r.dief("gh %s failed (%d):\n%s", strings.Join(args, " "), out.code,
			textutil.Truncate(firstNonEmpty(out.stderr, out.stdout), diagnosticLimit))
	}

	return out.stdout
}

// write puts a run's artifact where it can be read after the fact.
func (r *Run) write(path, content string) {
	err := os.MkdirAll(StateDir, dirMode)
	if err != nil {
		r.dief("creating %s: %v", StateDir, err)
	}

	err = os.WriteFile(path, []byte(content), fileMode)
	if err != nil {
		r.dief("writing %s: %v", path, err)
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

// envDuration reads a step's timeout override, in seconds.
func envDuration(name string, fallback time.Duration) time.Duration {
	seconds := envInt(name, int(fallback/time.Second))

	return time.Duration(seconds) * time.Second
}

// envInt reads a numeric override, falling back on anything unreadable.
func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}
