// Package reporter holds the harness's product-independent run record.
package reporter

import "time"

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
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	CLI       string    `json:"cli"`
	Model     string    `json:"model"`
	// InputHash fingerprints the prompt the judge was shown. Two runs
	// with one hash and two verdicts is judge noise, and now countable.
	InputHash    string         `json:"judge_input_hash,omitempty"`
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
