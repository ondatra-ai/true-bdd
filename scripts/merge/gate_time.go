package merge

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

// What CI calls the workflow and the job the gates run in: ci.yml's `name:`
// and `jobs.gates.name`, which is also the required check's name.
const (
	ciWorkflow = "CI"
	ciGatesJob = "gates"
)

// The two conclusions a step that actually executed can carry. A gate after a
// red one is stamped `skipped` at the failure's own instant, and reporting
// that as 0s would claim a gate ran in no time when it never ran at all.
const (
	ciSuccess = "success"
	ciFailure = "failure"
)

// The gh argv fragments this file repeats.
const (
	runVerb  = "run"
	apiVerb  = "api"
	jsonFlag = "--json"
	jqFlag   = "--jq"
)

// gateRun is one finished CI gates job, as much of it as the log wants.
type gateRun struct {
	id         int64
	sha        string
	conclusion string
	url        string
	total      time.Duration
	perGate    []string
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
		"gates", strings.Join(g.perGate, ", "),
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
	Steps       []ciStep  `json:"steps"`
}

// fold walks the gate table rather than the payload: the breakdown carries the
// pipeline's own rows in its own order, and CI's setup steps never appear.
// Timestamps are second-resolution, so a fast gate honestly reads 0s.
func (j ciJob) fold() gateRun {
	folded := gateRun{
		id:         j.RunID,
		sha:        j.HeadSHA,
		conclusion: j.Conclusion,
		url:        j.HTMLURL,
		total:      j.CompletedAt.Sub(j.StartedAt),
	}

	ran := make(map[string]ciStep, len(j.Steps))

	for _, step := range j.Steps {
		if step.ran() {
			ran[step.Name] = step
		}
	}

	for _, gate := range gates.All {
		step, executed := ran[gate.Name]
		if executed {
			folded.perGate = append(folded.perGate,
				fmt.Sprintf("%s %ds", gate.Name, step.seconds()))
		}
	}

	return folded
}

type ciStep struct {
	Name        string    `json:"name"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// ran reports whether the step executed, rather than being stamped in passing.
func (s ciStep) ran() bool {
	return s.Conclusion == ciSuccess || s.Conclusion == ciFailure
}

func (s ciStep) seconds() int64 {
	return int64(s.CompletedAt.Sub(s.StartedAt).Seconds())
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
	listed := r.sh([]string{
		ghBin, runVerb, "list", "--workflow", ciWorkflow, "--branch", branch,
		"--status", "completed", "--limit", "1",
		jsonFlag, "databaseId", jqFlag, ".[].databaseId",
	}, options{})

	runID := strings.TrimSpace(listed.stdout)
	if listed.code != 0 || runID == "" {
		slog.Info("No completed CI gates run to report", "branch", branch)

		return
	}

	jobs := r.sh([]string{ghBin, apiVerb,
		"repos/" + r.repo + "/actions/runs/" + runID + "/jobs"}, options{})

	timings, folded := gateTimings([]byte(jobs.stdout))
	if !folded {
		slog.Info("Could not read the CI gates run", "ci_run", runID)

		return
	}

	slog.Info("Last CI gates", timings.attrs()...)
}
