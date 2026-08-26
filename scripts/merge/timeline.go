package merge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ledgerPath is the commit run's stopwatch, written by
// .claude/skills/pr-commit/timings.sh: one `step<TAB>seconds` row per step.
const ledgerPath = "tmp/timings.tsv"

// Phase is one timed step of the run, named after the span tree in
// docs/for_further/observability.md wherever that tree names the operation.
// `split` and `dispose` are ours — the tree has no span for either.
type Phase struct {
	Name    string  `json:"name"`
	Round   int     `json:"round,omitempty"`
	Seconds float64 `json:"seconds"`
	// Outcome is what the phase already computed on its way out — its enum
	// where it has one, its counts where those are what it produces.
	Outcome string `json:"outcome,omitempty"`
}

// TimingReport is what tmp/merge/timings.json holds, beside the round
// artifacts. Exported so scripts/cmd/postmortem can read one back and render
// the same table a merge would have handed the model itself.
type TimingReport struct {
	PR            int     `json:"pr"`
	StartedAt     string  `json:"started_at"`
	CommitSeconds float64 `json:"commit_seconds"`
	TotalSeconds  float64 `json:"total_seconds"`
	Phases        []Phase `json:"phases"`
}

// Render is the table the postmortem is handed, and the one a run ends with.
func (report TimingReport) Render() string {
	var out strings.Builder

	_, _ = fmt.Fprintln(&out, "| Phase | Round | Seconds | Outcome |")
	_, _ = fmt.Fprintln(&out, "| --- | --- | --- | --- |")

	if report.CommitSeconds > 0 {
		_, _ = fmt.Fprintf(&out, "| the commit that produced this PR |  | %.0f | %s |\n",
			report.CommitSeconds, ledgerPath)
	}

	for _, entry := range report.Phases {
		_, _ = fmt.Fprintf(&out, "| %s | %s | %.1f | %s |\n",
			entry.Name, roundLabel(entry.Round), entry.Seconds, entry.Outcome)
	}

	_, _ = fmt.Fprintf(&out, "| **total** |  | **%.1f** |  |\n", report.TotalSeconds)

	return out.String()
}

func roundLabel(round int) string {
	if round == 0 {
		return ""
	}

	return strconv.Itoa(round)
}

// timeline is this run's clock plus what the commit before it already spent.
// Mutated for the length of the run, so Run holds a pointer to it.
type timeline struct {
	started time.Time
	// before is the ledger as it stood at Start, snapshotted rather than
	// re-read: commit() runs gates.sh again, which appends to the same file.
	before time.Duration
	phases []Phase
}

func newTimeline(before time.Duration) *timeline {
	return &timeline{started: time.Now(), before: before}
}

func (t *timeline) add(entry Phase) { t.phases = append(t.phases, entry) }

// total is the commit that produced the PR plus this run so far — what §3 of
// the ticket measures, not every minute task-handle spent. A task's Work and
// Review steps are agent turns no shell brackets, so they are not in here.
func (t *timeline) total() time.Duration {
	return t.before + time.Since(t.started)
}

// step times one phase. The returned function records it, taking the outcome
// the phase computed on its way to it.
func (r *Run) step(name string, round int) func(outcome string) {
	started := time.Now()

	return func(outcome string) {
		r.timings.add(Phase{
			Name:    name,
			Round:   round,
			Seconds: time.Since(started).Seconds(),
			Outcome: outcome,
		})
	}
}

// timingReport is the run so far, as the record and the table both read it.
func (r *Run) timingReport() TimingReport {
	return TimingReport{
		PR:            r.pr,
		StartedAt:     r.startedAt,
		CommitSeconds: r.timings.before.Seconds(),
		TotalSeconds:  r.timings.total().Seconds(),
		Phases:        r.timings.phases,
	}
}

// reportTimings persists the record and prints the table the run ends with.
func (r *Run) reportTimings() {
	r.persistTimings()
	r.banner("timings")

	_, _ = fmt.Fprintln(os.Stdout, "\n"+r.timingReport().Render())
}

// persistTimings writes the record beside the round artifacts. Errors are
// swallowed on purpose: it is also called from dief, where a stop is already
// being reported and a second one would bury it.
func (r *Run) persistTimings() {
	if r.timings == nil {
		return
	}

	encoded, err := json.MarshalIndent(r.timingReport(), "", "  ")
	if err != nil {
		return
	}

	if os.MkdirAll(StateDir, dirMode) == nil {
		_ = os.WriteFile(StateDir+"/timings.json", encoded, fileMode)
	}
}

// readLedger sums the commit run's ledger. A missing or malformed file is
// zero — timing must never be able to stop a merge.
func readLedger(path string) time.Duration {
	handle, err := os.Open(path) //nolint:gosec // a constant path under the repository root.
	if err != nil {
		return 0
	}

	defer handle.Close() //nolint:errcheck // read-only.

	var total float64

	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		_, field, found := strings.Cut(scanner.Text(), "\t")
		if !found {
			continue
		}

		seconds, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
		if err == nil {
			total += seconds
		}
	}

	return time.Duration(total * float64(time.Second))
}
