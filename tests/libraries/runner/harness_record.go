package runner

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// HarnessRecordFile is the record's name inside SpawnLogDir.
const HarnessRecordFile = "harness.json"

// harnessSchema versions the on-disk shape: 2 added the judge transcript
// and manifest snapshot; 3 added the AI-CLI mode. A reader that doesn't
// recognise a schema should say so, not render fields it guessed at.
const harnessSchema = 3

// The sidecar files the recorder writes beside the record, all inside
// SpawnLogDir.
const (
	// JudgeSystemFile, JudgeUserFile and JudgeResponseFile are the judge
	// call verbatim, kept as separate files rather than record fields: the
	// user prompt runs to tens of KB and would make the record unreadable.
	JudgeSystemFile   = "judge-system.txt"
	JudgeUserFile     = "judge-user.txt"
	JudgeResponseFile = "judge-response.txt"
	// ManifestSnapshotFile is the fixture's resolved manifest as this run
	// saw it — snapshotted because fixture.yaml is never copied into the
	// tmpdir (see ObserveFixture for why that matters across runs).
	ManifestSnapshotFile = "manifest.json"
)

// The verdict words, matching what `go test -v` prints for a subtest —
// they are the run report's existing vocabulary.
const (
	VerdictPass = "PASS"
	VerdictFail = "FAIL"
	VerdictSkip = "SKIP"
)

// FileChangeRecord is one entry of the run's diff without its content:
// what changed, where, and how big it ended up. Content is deliberately
// absent — already on disk in the preserved tmpdir and handed to the judge.
type FileChangeRecord struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	BytesBefore int    `json:"bytes_before"`
	BytesAfter  int    `json:"bytes_after"`
}

// JudgeRecord is the harness judge's single model call. EndedAt is the
// far edge on purpose: the report measures the trailing harness block
// from the engine's last log record to the judge's end.
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
// the test process — the engine's own JSON log covers its own process;
// this covers what it cannot see: verdict, duration, diff, judge cost.
type HarnessRecord struct {
	Schema  int    `json:"schema"`
	Fixture string `json:"fixture"`
	// Mode is the AI-CLI mode this fixture ran under: live, record or
	// replay. Empty for a pre-shim session. Carried per fixture as well as
	// per session so the two can be seen to disagree if they ever do.
	Mode      string    `json:"mode,omitempty"`
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
	// Artifacts names the sidecar files this run actually wrote, so a
	// reader can tell "the judge was never asked" from "this session
	// predates the transcript" without probing the filesystem.
	Artifacts []string `json:"artifacts,omitempty"`
}

// ManifestSnapshot is the fixture manifest as one run resolved it: what
// the run was asked to do, and what it was asserted against.
type ManifestSnapshot struct {
	Name          string   `json:"name"`
	Cmd           string   `json:"cmd"`
	InputPath     string   `json:"input_path"`
	Answers       string   `json:"answers,omitempty"`
	PrepCmds      []string `json:"prep_cmds,omitempty"`
	TeardownCmds  []string `json:"teardown_cmds,omitempty"`
	ExpectedExit  int      `json:"expected_exit_code"`
	StdoutRegexes []string `json:"stdout_regexes,omitempty"`
	JudgeSpec     string   `json:"judge_spec"`
	TimeoutMs     int64    `json:"timeout_ms,omitempty"`
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

	// sidecars are the extra files to write beside the record, by name.
	// Accumulated rather than written as they arrive, so every filesystem
	// touch happens in Finish (see writeHarnessRecord for why that matters).
	sidecars map[string]string
}

// NewHarnessRecorder starts the fixture's clock. The directory is
// derived from sessionRoot and the name, not RunResult.TmpDir — a
// fixture that fails inside prepareRunDir has an empty TmpDir but is still worth recording.
func NewHarnessRecorder(sessionRoot, name, mode string, usage *UsageSink) *HarnessRecorder {
	started := time.Now()

	return &HarnessRecorder{
		record:   HarnessRecord{Schema: harnessSchema, Fixture: name, Mode: mode, StartedAt: started},
		started:  started,
		dir:      filepath.Join(RunDir(sessionRoot, name), SpawnLogDir),
		usage:    usage,
		sidecars: map[string]string{},
	}
}

// ObserveFixture snapshots the manifest this run resolved, rather than
// reading it back from the source tree at report time — fixture.yaml
// isn't copied into the tmpdir, so a later read would show today's rubric.
func (r *HarnessRecorder) ObserveFixture(fixture *Fixture) {
	if fixture == nil {
		return
	}

	patterns := make([]string, 0, len(fixture.StdoutRegexes))
	for _, pattern := range fixture.StdoutRegexes {
		patterns = append(patterns, pattern.String())
	}

	snapshot := ManifestSnapshot{
		Name:          fixture.Name,
		Cmd:           fixture.Cmd,
		InputPath:     fixture.InputPath,
		Answers:       string(fixture.Stdin),
		PrepCmds:      fixture.PrepCmds,
		TeardownCmds:  fixture.TeardownCmds,
		ExpectedExit:  fixture.ExpectedExitCode,
		StdoutRegexes: patterns,
		JudgeSpec:     fixture.JudgeSpec,
		TimeoutMs:     fixture.Timeout.Milliseconds(),
	}

	blob, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		slog.Warn("BDD runner: cannot encode manifest snapshot", "error", err)

		return
	}

	r.sidecars[ManifestSnapshotFile] = string(blob) + "\n"
}

// ObserveRun folds in what the CLI invocation left behind. The exit
// code is recorded only when Execute got far enough to have one — a
// died-in-prep fixture has none, and 0 would lie as "it worked".
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

	// Only what the judge actually produced. An empty file would read as
	// "the judge was asked and said nothing", which is a different fact
	// from "the judge was never reached".
	for name, body := range map[string]string{
		JudgeSystemFile:   verdict.JudgeSystemPrompt,
		JudgeUserFile:     verdict.JudgeUserPrompt,
		JudgeResponseFile: verdict.JudgeResponse,
	} {
		if body != "" {
			r.sidecars[name] = body
		}
	}
}

// AddFailure records a check the fixture did not survive to make. The
// harness gives up in places that never produce a Verdict; without this
// those write a FAIL with no reason.
func (r *HarnessRecorder) AddFailure(msg string) {
	r.record.Failures = append(r.record.Failures, msg)
}

// Finish stamps the wall clock, bills the judge, and writes the record.
// The stamp is taken FIRST — everything below (sink scan, marshal,
// write) is the recorder's own cost, not billable to the fixture.
func (r *HarnessRecorder) Finish(failed, skipped bool) string {
	ended := time.Now()

	// ended.Sub uses both stamps' monotonic readings, so this is a real
	// elapsed time and not the difference of two wall clocks that an NTP
	// step could have moved apart mid-fixture.
	r.record.EndedAt = ended
	r.record.WallMs = ended.Sub(r.started).Milliseconds()
	r.record.Verdict = verdictWord(failed, skipped)

	r.billJudge()

	// Sidecars first: the record names them, and a record that advertises
	// a file the reader cannot open is worse than one that admits nothing
	// was written. Only names that actually landed are recorded.
	r.record.Artifacts = writeSidecars(r.dir, r.sidecars)

	return writeHarnessRecord(r.dir, r.record)
}

// writeSidecars persists the judge transcript and manifest snapshot,
// returning the names that landed, sorted. Safe for the same reason
// writeHarnessRecord is (see its doc): Finish runs after both snapshots.
func writeSidecars(dir string, bodies map[string]string) []string {
	if len(bodies) == 0 {
		return nil
	}

	err := disk.Dir(dir, disk.Shared)
	if err != nil {
		slog.Warn("BDD runner: cannot create", "path", dir, "error", err)

		return nil
	}

	written := make([]string, 0, len(bodies))

	for name, body := range bodies {
		path := filepath.Join(dir, name)

		writeErr := disk.Write(path, []byte(body), disk.Shared)
		if writeErr != nil {
			slog.Warn("BDD runner: cannot write", "path", path, "error", writeErr)

			continue
		}

		written = append(written, name)
	}

	sort.Strings(written)

	return written
}

// billJudge claims the usage records stamped inside this fixture's
// judge window: fixtures run sequentially, so no record can belong to
// two (see Between and TestUsageOutsideJudgeWindowIsNotBilled).
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

// writeHarnessRecord persists one fixture's record. SAFE only after
// Execute's post-run snapshot (else harness.json enters the judge's
// diff); errors are reported and swallowed, never failing the fixture.
func writeHarnessRecord(dir string, record HarnessRecord) string {
	err := disk.Dir(dir, disk.Shared)
	if err != nil {
		slog.Warn("BDD runner: cannot create", "path", dir, "error", err)

		return ""
	}

	blob, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		slog.Warn("BDD runner: cannot encode harness record", "error", err)

		return ""
	}

	path := filepath.Join(dir, HarnessRecordFile)

	// This file's presence tells the report server "fixture final, cache it
	// forever", so disk.Write's rename is what stops it caching a truncated
	// harness.json — a plain write leaves a valid-looking partial JSON.
	err = disk.Write(path, append(blob, '\n'), disk.Shared)
	if err != nil {
		slog.Warn("BDD runner: cannot write", "path", path, "error", err)

		return ""
	}

	return path
}
