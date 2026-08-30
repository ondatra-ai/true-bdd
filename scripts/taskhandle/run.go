package taskhandle

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"github.com/ondatra-ai/true-bdd/scripts/config"
	"github.com/ondatra-ai/true-bdd/scripts/report"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// StateDir holds what a run produces, so a plan and an outcome can be read
// after it.
const StateDir = "tmp/task-handle"

const (
	gitBin = "git"
	ghBin  = "gh"
	trunk  = "main"
)

var (
	errUsage      = errors.New("usage: task-handle <ticket-id>")
	errNotARepo   = errors.New("not inside a git repository")
	errNotOnTrunk = errors.New("not on " + trunk)
	errDirtyTree  = errors.New("the working tree is dirty")
	errMissing    = errors.New("not found in PATH")
)

// Run is one Ticket, from one checkout. Everything Start settles is fixed for
// the whole run; the rest is filled in as the steps reach it.
type Run struct {
	ticketID string
	detail   Detail
	repo     string

	budget *budget
	list   *checklist

	// reviewEnabled is scripts/.config.json's code_review switch.
	reviewEnabled bool

	pullRequest string
	sha         string
}

// Start settles every precondition once, up front, and returns them as errors
// rather than exiting — a parent's halting protocol has to be able to run.
func Start(args []string) (*Run, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return nil, errUsage
	}

	err := enterRepo()
	if err != nil {
		return nil, err
	}

	err = requireTools()
	if err != nil {
		return nil, err
	}

	err = requireCleanTrunk()
	if err != nil {
		return nil, err
	}

	// Read up front: a config that does not parse must stop the run before the
	// mandate is stamped, not between two turns that have already edited.
	switches, err := config.Load(config.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", config.Path, err)
	}

	return &Run{
		ticketID:      strings.TrimSpace(args[0]),
		repo:          ".",
		budget:        newBudget(),
		list:          newChecklist(),
		reviewEnabled: config.On(switches.CodeReview),
	}, nil
}

func enterRepo() error {
	top, err := git.TopLevel()
	if err != nil {
		return errNotARepo
	}

	err = os.Chdir(top)
	if err != nil {
		return fmt.Errorf("entering the repository root: %w", err)
	}

	return nil
}

func requireTools() error {
	err := cli.Require(gitBin, ghBin, "claude")
	if err != nil {
		return fmt.Errorf("%w: %w", errMissing, err)
	}

	return nil
}

// requireCleanTrunk is the refusal task-handle's skill used to carry as prose.
// In Go it cannot be skipped, and it runs before anything is written.
func requireCleanTrunk() error {
	branch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("reading the current branch: %w", err)
	}

	if branch != trunk {
		return fmt.Errorf("%w — on %q; a Ticket starts from %s", errNotOnTrunk, branch, trunk)
	}

	dirty, err := git.ShortStatus()
	if err != nil {
		return fmt.Errorf("reading the worktree: %w", err)
	}

	if strings.TrimSpace(dirty) != "" {
		return fmt.Errorf("%w:\n%s", errDirtyTree, dirty)
	}

	return nil
}

// Main walks the Ticket and reports. It returns the outcome rather than
// exiting: every one of the five is a verdict the command produced.
func Main(run *Run) Outcome {
	defer run.finish()

	outcome := run.settle(run.walk())

	run.report(outcome)
	run.logRun(outcome)

	return outcome
}

// walk is steps 1 to 8. Split in two so neither half argues with cyclop.
func (r *Run) walk() error {
	err := r.prepare()
	if err != nil {
		return err
	}

	return r.deliver()
}

// prepare is steps 1 to 4: read the Ticket, take it, do the work, check it
// stayed inside what the Ticket asked for.
func (r *Run) prepare() error {
	steps := []func() error{r.check, r.begin, r.work, r.scope}
	for _, step := range steps {
		err := step()
		if err != nil {
			return err
		}
	}

	return nil
}

// deliver is steps 5 to 8: commit, review, merge, close.
func (r *Run) deliver() error {
	steps := []func() error{r.commitStep, r.reviewStep, r.mergeStep, r.closeStep}
	for _, step := range steps {
		err := step()
		if err != nil {
			return err
		}
	}

	return nil
}

// finish renders this run's own tree, out of the log it has been writing all
// along — which makes every run a round-trip test of the log's structure.
func (r *Run) finish() {
	report.Render(state.TaskLog("."), logging.Run(), StateDir+"/report.md")
}

func (r *Run) logf(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

// unmandate withdraws the authority to merge. It runs on every terminal path
// except a clean DONE, and FIRST on the declining one: a crash between the two
// writes must not leave standing merge authority.
func (r *Run) unmandate() {
	err := state.Set(r.repo, state.MandateKey, "")
	if err != nil {
		slog.Error("could not clear the mandate", "error", err)
	}
}

func (r *Run) unbind() {
	err := state.Set(r.repo, state.TicketKey, "")
	if err != nil {
		slog.Error("could not clear the Ticket binding", "error", err)
	}
}

// settle turns what stopped the run into one of the five verdicts, and writes
// whatever that verdict owes ClickUp.
func (r *Run) settle(err error) Outcome {
	switch {
	case err == nil:
		return OutcomeDone
	case errors.Is(err, errMandateRevoked):
		r.unmandate()

		return OutcomeAwaitingMerge
	case isDecline(err):
		return r.settleDecline(err)
	case isNotStarted(err):
		// Step 1 wrote nothing and must not start now: the Ticket stays in
		// TO DO, untouched, for a person to fill in.
		return OutcomeNotStarted
	default:
		return r.settleHalt(err)
	}
}

// settleDecline refuses on the merits. Unmandate first, then the status, then
// the unbind — clickup.CloseBound holds the last two in that order.
func (r *Run) settleDecline(err error) Outcome {
	r.unmandate()

	var declined *declineError

	_ = errors.As(err, &declined)

	_, closeErr := clickup.CloseBound(r.repo, "FAILED", declined.reason)
	if closeErr != nil {
		slog.Error("the Ticket could not be closed FAILED", "error", closeErr)
	}

	return OutcomeFailed
}

// settleHalt stops where it stands. The binding is LEFT so /task-done and
// /task-fail can read what is being closed — except at step 2, where the bind
// is one this run just wrote and a bound TO DO Ticket is handed out again.
func (r *Run) settleHalt(err error) Outcome {
	r.unmandate()

	var halted *haltError
	if errors.As(err, &halted) && halted.step == StepStart {
		r.unbind()
	}

	slog.Error("STOPPED: " + err.Error())

	return OutcomeHalted
}

// envDuration reads a step's timeout override, in seconds.
func envDuration(name string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(os.Getenv(name))
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}
