package commit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/config"
	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const (
	gitBin = "git"
	ghBin  = "gh"

	// The trunk, and the base a narrowed gate selection diffs against.
	trunk    = "main"
	altTrunk = "master"

	// StateDir holds what a run produces, so a message can be read after it.
	StateDir = "tmp/commit"

	diagnosticLimit = 800
)

// Run is one invocation, from one checkout. Every field is settled by Start.
type Run struct {
	// unattended reports that task-handle stamped a mandate: nobody is at the
	// terminal, so the gates narrow to the ones this diff needs.
	unattended bool

	// The two skill turns scripts/.config.json can switch off.
	docUniverseEnabled  bool
	updateMemoryEnabled bool
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

	// Read up front: a config that does not parse must stop the run before the
	// gates, not between two skill turns that have already edited the tree.
	switches, err := config.Load(config.Path)
	if err != nil {
		usage(err.Error())
	}

	return &Run{
		unattended:          state.Get(".", state.MandateKey) != "",
		docUniverseEnabled:  config.On(switches.DocUniverse),
		updateMemoryEnabled: config.On(switches.UpdateMemory),
	}
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

	slog.Info("Pull request", "url", r.UpdatePR())

	r.finish()
}

// finish renders this run's own tree, out of the log it has been writing all
// along — which makes every run a round-trip test of the log's structure.
func (r *Run) finish() {
	report.Render(state.TaskLog("."), logging.Run(), StateDir+"/report.md")
}

// gates runs the quality pipeline. Under a mandate it narrows to the gates the
// diff needs, which is ~2s on a documentation ticket against ~140s; selection
// is LOCAL ONLY, so CI still catches whatever the selector skipped.
func (r *Run) gates() {
	defer report.Open("gates")()

	selected := gates.All

	if r.unattended {
		started := time.Now()

		changed, err := gates.Changed(r.trunkRef())
		if err != nil {
			r.dief("selecting gates: %v", err)
		}

		selected = gates.Select(changed)
		report.Leaf("select gates", started, "selected", len(selected),
			"total", len(gates.All), "changed", len(changed), "base", r.trunkRef())
	}

	err := gates.Run(selected)
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
	slog.Info(fmt.Sprintf(format, args...))
}

// dief stops the run with a diagnosis. Nothing here is swallowed. The ERROR is
// logged BEFORE the report is folded: it is what tells the operations left open
// apart from a run that was killed outright.
func (r *Run) dief(format string, args ...any) {
	slog.Error("STOPPED: " + fmt.Sprintf(format, args...))
	r.finish()
	os.Exit(1)
}

// usage stops before the run has a context to report against.
func usage(message string) {
	slog.Error(message)
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
		command.Stdout, command.Stderr = console.Out(), console.Err()
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
	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		r.dief("%v", err)
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
