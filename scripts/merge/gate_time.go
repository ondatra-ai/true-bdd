package merge

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/github"
)

// What CI calls the workflow and the job the gates run in: ci.yml's `name:`
// and `jobs.gates.name`, which is also the required check's name.
const (
	ciWorkflow = "CI"
	ciGatesJob = "gates"
)

// gateRun is one finished CI gates job, as much of it as the log wants.
type gateRun struct {
	id         int64
	sha        string
	conclusion string
	url        string
	total      time.Duration
}

// attrs is the record's attributes, and none of report's keys: with no `tree`
// and no `duration_ms` this stays prose the report ignores. `run` is the
// process id pkg/logging stamps, so CI's run is `ci_run`.
func (g gateRun) attrs() []any {
	return []any{
		"ci_run", g.id,
		"sha", short(g.sha),
		"conclusion", g.conclusion,
		"seconds", int64(g.total.Seconds()),
		"url", g.url,
	}
}

// ciJobs is `gh api repos/<repo>/actions/runs/<id>/jobs`.
type ciJobs struct {
	Jobs []ciJob `json:"jobs"`
}

type ciJob struct {
	RunID       int64     `json:"run_id"`
	Name        string    `json:"name"`
	Conclusion  string    `json:"conclusion"`
	HeadSHA     string    `json:"head_sha"`
	HTMLURL     string    `json:"html_url"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// fold reads the gates job's own wall clock. There is no per-gate breakdown
// to read: ci.yml runs the pipeline as ONE step so the gate table stays the
// only place gates are listed, and GitHub times steps, not what runs inside.
func (j ciJob) fold() gateRun {
	return gateRun{
		id:         j.RunID,
		sha:        j.HeadSHA,
		conclusion: j.Conclusion,
		url:        j.HTMLURL,
		total:      j.CompletedAt.Sub(j.StartedAt),
	}
}

// gateTimings folds the jobs payload into the gates job's own wall clock.
func gateTimings(payload []byte) (gateRun, bool) {
	var decoded ciJobs

	err := json.Unmarshal(payload, &decoded)
	if err != nil {
		return gateRun{}, false
	}

	for _, job := range decoded.Jobs {
		if job.Name == ciGatesJob {
			return job.fold(), true
		}
	}

	return gateRun{}, false
}

// reportGateTime drops the last completed CI gates job's wall clock into the
// Task log. A note in the margin, not a step: every failure is one Info line,
// because a WARN would mark whichever node the record lands in.
func (r *Run) reportGateTime(branch string) {
	runID, err := github.LastCompletedRun(ciWorkflow, branch)
	if err != nil || runID == "" {
		slog.Info("No completed CI gates run to report", "branch", branch)

		return
	}

	jobs, err := github.RunJobs(r.repo, runID)
	if err != nil {
		slog.Info("Could not read the CI gates run", "ci_run", runID)

		return
	}

	timings, folded := gateTimings(jobs)
	if !folded {
		slog.Info("Could not read the CI gates run", "ci_run", runID)

		return
	}

	slog.Info("Last CI gates", timings.attrs()...)
}
