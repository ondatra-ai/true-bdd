package commit

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/config"
	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const (
	// remote is the only one this repository pushes to or reads refs from.
	remote = "origin"

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

	// embedded reports that a parent imported this package and folds the whole
	// tree itself, so this run must not render a partial one over the top.
	embedded bool

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

	top, err := git.TopLevel()
	if err != nil {
		usage("not inside a git repository")
	}

	err = os.Chdir(top)
	if err != nil {
		usage("cannot enter the repository root: " + err.Error())
	}

	err = cli.Require(git.Bin, github.Bin, claude.Bin)
	if err != nil {
		usage(err.Error())
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
	if r.embedded {
		return
	}

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
		r.dief("%s\n  %v", GatesRedPrefix, err)
	}
}

// trunkRef is main, or master where that is what the repository calls it.
func (r *Run) trunkRef() string {
	for _, name := range []string{trunk, altTrunk} {
		exists, err := git.RemoteBranchExists(remote, name)
		if err == nil && exists {
			return name
		}
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
	message := fmt.Sprintf(format, args...)

	slog.Error("STOPPED: " + message)
	r.finish()
	panic(stopSentinel{message: message})
}

// usage stops before the run has a context to report against.
func usage(message string) {
	slog.Error(message)
	panic(stopSentinel{message: message})
}

// -------------------------------------------------------------- stop policy

// check stops the run when an operation failed. This is where commit's policy
// lives: a git or gh command that failed ends the run, which is what the
// gitChecked/gh pair this replaces each said for itself.
func (r *Run) check(what string, err error) {
	if err != nil {
		r.dief("%s: %v", what, textutil.Truncate(err.Error(), diagnosticLimit))
	}
}

// write puts a run's artifact where it can be read after the fact.
func (r *Run) write(path, content string) {
	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		r.dief("%v", err)
	}
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
