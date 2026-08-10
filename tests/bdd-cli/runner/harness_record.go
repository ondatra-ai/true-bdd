package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HarnessRecordFile is the record's name inside SpawnLogDir.
const HarnessRecordFile = "harness.json"

// harnessSchema versions the on-disk shape. A reader that does not
// recognise it should say so rather than render a report from fields it
// guessed at.
const harnessSchema = 1

// The verdict words, matching what `go test -v` prints for a subtest —
// they are the run report's existing vocabulary.
const (
	VerdictPass = "PASS"
	VerdictFail = "FAIL"
	VerdictSkip = "SKIP"
)

// FileChangeRecord is one entry of the run's diff without its content:
// what changed, where, and how big it ended up.
//
// Content is deliberately absent. The bodies are already handed to the
// judge and already on disk inside the preserved tmpdir; copying them
// here would make the record grow with the size of whatever the model
// wrote, for a report that renders one line per entry.
type FileChangeRecord struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	BytesBefore int    `json:"bytes_before"`
	BytesAfter  int    `json:"bytes_after"`
}

// JudgeRecord is the harness judge's single model call.
//
// StartedAt/EndedAt bracket the call itself. EndedAt is the far edge on
// purpose: the report measures the trailing harness block as the span
// from the engine's last log record to the end of the judge, so the near
// edge would leave the model call unaccounted for.
type JudgeRecord struct {
	StartedAt    time.Time      `json:"started_at"`
	EndedAt      time.Time      `json:"ended_at"`
	CLI          string         `json:"cli"`
	Model        string         `json:"model"`
	CostUSD      float64        `json:"cost_usd"`
	Tokens       int            `json:"tokens"`
	TokensByKind map[string]int `json:"tokens_by_kind,omitempty"`
}

// HarnessRecord is everything about one fixture run that exists only in
// the test process. The engine writes its own JSON log from inside its
// own process; this is the same idea one level up, for the four facts
// the engine cannot see: what the harness decided, how long the whole
// fixture took, what changed on disk, and what the judge spent.
type HarnessRecord struct {
	Schema    int       `json:"schema"`
	Fixture   string    `json:"fixture"`
	Verdict   string    `json:"verdict"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	// WallMs is a MONOTONIC elapsed time, not EndedAt-StartedAt: a clock
	// step mid-fixture must not be able to move it. Milliseconds, to
	// match the engine log's own duration_ms vocabulary.
	WallMs int64 `json:"wall_ms"`
	// ExitCode is nil when the CLI never got as far as exiting.
	ExitCode   *int               `json:"exit_code"`
	Failures   []string           `json:"failures,omitempty"`
	Diff       []FileChangeRecord `json:"diff"`
	Judge      JudgeRecord        `json:"judge"`
	TmpDir     string             `json:"tmp_dir"`
	StdoutFile string             `json:"stdout_file,omitempty"`
	StderrFile string             `json:"stderr_file,omitempty"`
}

// RunDir is where one fixture's tmpdir lives inside a session. The
// harness recorder and prepareRunDir must agree on this exactly, so
// neither computes it itself.
func RunDir(sessionRoot, name string) string {
	return filepath.Join(sessionRoot, name)
}

// HarnessRecorder accumulates one fixture's record and writes it when
// the subtest ends.
type HarnessRecorder struct {
	record  HarnessRecord
	started time.Time
	dir     string
	usage   *UsageSink
}

// NewHarnessRecorder starts the fixture's clock.
//
// The directory is derived from sessionRoot and the fixture name rather
// than taken from RunResult.TmpDir, because a fixture that fails inside
// prepareRunDir returns an empty TmpDir — and that is precisely a run
// worth recording.
func NewHarnessRecorder(sessionRoot, name string, usage *UsageSink) *HarnessRecorder {
	started := time.Now()

	return &HarnessRecorder{
		record:  HarnessRecord{Schema: harnessSchema, Fixture: name, StartedAt: started},
		started: started,
		dir:     filepath.Join(RunDir(sessionRoot, name), SpawnLogDir),
		usage:   usage,
	}
}

// ObserveRun folds in what the CLI invocation left behind.
//
// The exit code is recorded only when Execute got far enough to have
// one: a fixture that died in prep has no exit status, and reporting the
// zero value as "exit 0" would be a lie in the direction of "it worked".
func (r *HarnessRecorder) ObserveRun(result *RunResult, err error) {
	if result == nil {
		return
	}

	r.record.TmpDir = result.TmpDir
	r.record.StdoutFile = result.StdoutFile
	r.record.StderrFile = result.StderrFile
	r.record.Diff = diffRecords(result.Diff)

	if err == nil {
		code := result.ExitCode
		r.record.ExitCode = &code
	}
}

// ObserveVerdict folds in the three checks and the window the judge's
// model call occupied.
func (r *HarnessRecorder) ObserveVerdict(verdict Verdict) {
	r.record.Failures = append(r.record.Failures, verdict.Failures...)
	r.record.Judge.StartedAt = verdict.JudgeStartedAt
	r.record.Judge.EndedAt = verdict.JudgeEndedAt
	r.record.Judge.CLI = "claude"
	r.record.Judge.Model = judgeModel
}

// AddFailure records a check the fixture did not survive to make. The
// harness gives up in places that never produce a Verdict; without this
// those write a FAIL with no reason.
func (r *HarnessRecorder) AddFailure(msg string) {
	r.record.Failures = append(r.record.Failures, msg)
}

// Finish stamps the wall clock, bills the judge, and writes the record.
//
// The stamp is taken FIRST. Everything below it — the sink scan, the
// marshal, the mkdir, the write — is the recorder's own cost, and
// charging that to the fixture would push it into the report's one
// residual slice, where it would be indistinguishable from prep.
func (r *HarnessRecorder) Finish(failed, skipped bool) string {
	ended := time.Now()

	// ended.Sub uses both stamps' monotonic readings, so this is a real
	// elapsed time and not the difference of two wall clocks that an NTP
	// step could have moved apart mid-fixture.
	r.record.EndedAt = ended
	r.record.WallMs = ended.Sub(r.started).Milliseconds()
	r.record.Verdict = verdictWord(failed, skipped)

	r.billJudge()

	return writeHarnessRecord(r.dir, r.record)
}

// billJudge claims the usage records stamped inside this fixture's judge
// window. Fixtures run sequentially and the window brackets the model
// call itself, so no record can belong to two fixtures — and a fixture
// that died before reaching its judge claims nothing, rather than
// inheriting the next one's cost.
func (r *HarnessRecorder) billJudge() {
	if r.usage == nil || r.record.Judge.StartedAt.IsZero() {
		return
	}

	cost, tokens := r.usage.Between(r.record.Judge.StartedAt, r.record.Judge.EndedAt)

	r.record.Judge.CostUSD = cost
	r.record.Judge.TokensByKind = tokens

	for _, count := range tokens {
		r.record.Judge.Tokens += count
	}
}

// verdictWord maps the test's own state onto what `go test` would have
// printed for the subtest.
func verdictWord(failed, skipped bool) string {
	switch {
	case skipped:
		return VerdictSkip
	case failed:
		return VerdictFail
	default:
		return VerdictPass
	}
}

// diffRecords projects the run's file changes onto their sizes.
func diffRecords(changes []FileChange) []FileChangeRecord {
	records := make([]FileChangeRecord, 0, len(changes))

	for _, change := range changes {
		records = append(records, FileChangeRecord{
			Kind:        change.Kind,
			Path:        change.Path,
			BytesBefore: len(change.Before),
			BytesAfter:  len(change.After),
		})
	}

	return records
}

// writeHarnessRecord persists one fixture's record and returns its path.
//
// It lands in SpawnLogDir for the same reason the CLI's streams do:
// evidence next to the run it explains. It is SAFE there only because
// this runs after Execute has taken BOTH snapshots (the "before" at the
// snapshotTree call preceding the CLI exec, the "after" at the one
// following it). Calling it from inside Execute, before the post-run
// snapshot, would put harness.json into the diff the judge grades.
//
// A recorder that cannot write must never fail a fixture, so every error
// here is reported and swallowed — same contract as the spawn log.
func writeHarnessRecord(dir string, record HarnessRecord) string {
	err := os.MkdirAll(dir, spawnLogDirPerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot create %s: %v\n", dir, err)

		return ""
	}

	blob, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot encode harness record: %v\n", err)

		return ""
	}

	path := filepath.Join(dir, HarnessRecordFile)

	err = os.WriteFile(path, append(blob, '\n'), spawnLogFilePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "BDD runner: cannot write %s: %v\n", path, err)

		return ""
	}

	return path
}
