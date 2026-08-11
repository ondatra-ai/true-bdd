package reportserver

// The JSON contract. Deliberately a separate layer from reporter's
// loader structs rather than tags on them:
//
//   - Phase carries a *Turn back-pointer; marshalling the loader types
//     directly would emit every turn twice, once in the turn list and
//     once nested in its phase, artifacts and all.
//   - Artifact holds whole prompt bodies, tens of kilobytes each. They
//     must never ride along in a list response; here they are references
//     and the body is a separate request.
//   - time.Duration marshals as an int64 of nanoseconds. Every duration
//     crosses this boundary as seconds, named so.
//   - Tagging the loaders would weld their field names to the wire, and
//     those files are pinned by the phase-timeline invariant tests.

// StateResponse is the poll target: small, memory-only, and enough to
// decide whether anything needs re-fetching.
type StateResponse struct {
	Version   int64        `json:"version"`
	ScannedAt string       `json:"scanned_at"`
	RunsDir   string       `json:"runs_dir"`
	Runs      []RunSummary `json:"runs"`
}

// RunSummary is one session as the run list shows it.
type RunSummary struct {
	ID           string  `json:"id"`
	Complete     bool    `json:"complete"`
	Fixtures     int     `json:"fixtures"`
	Passed       int     `json:"passed"`
	Failed       int     `json:"failed"`
	Skipped      int     `json:"skipped"`
	Turns        int     `json:"turns"`
	Tokens       int     `json:"tokens"`
	WallSeconds  float64 `json:"wall_seconds"`
	CostUSD      float64 `json:"cost_usd"`
	JudgeCostUSD float64 `json:"judge_cost_usd"`
}

// RunResponse is one run's page: its tests plus where to go next.
type RunResponse struct {
	Version int64         `json:"version"`
	Run     RunSummary    `json:"run"`
	Prev    string        `json:"prev"`
	Next    string        `json:"next"`
	Tests   []TestSummary `json:"tests"`
}

// RunListResponse is every run, oldest first.
type RunListResponse struct {
	Version int64        `json:"version"`
	Runs    []RunSummary `json:"runs"`
}

// TestSummary is one fixture row.
type TestSummary struct {
	Name    string `json:"name"`
	Verdict string `json:"verdict"`
	Command string `json:"command"`
	// HasRecord distinguishes "the harness recorded nothing" from "the
	// run genuinely did nothing", which look identical in the numbers.
	HasRecord         bool     `json:"has_record"`
	WallSeconds       float64  `json:"wall_seconds"`
	HasWall           bool     `json:"has_wall"`
	PhaseTotalSeconds float64  `json:"phase_total_seconds"`
	DriftSeconds      float64  `json:"drift_seconds"`
	Turns             int      `json:"turns"`
	ModelSeconds      float64  `json:"model_seconds"`
	CostUSD           float64  `json:"cost_usd"`
	JudgeCostUSD      float64  `json:"judge_cost_usd"`
	Tokens            int      `json:"tokens"`
	ExitCode          *int     `json:"exit_code"`
	DiffCount         int      `json:"diff_count"`
	Failures          []string `json:"failures"`
	ManifestSource    string   `json:"manifest_source"`
}

// TestDetail is one fixture's page.
type TestDetail struct {
	Version int64  `json:"version"`
	Run     string `json:"run"`
	// RunDir is the fixture's working directory relative to the repo
	// root — the one path everything else on this page hangs off.
	RunDir    string          `json:"run_dir"`
	Summary   TestSummary     `json:"summary"`
	Expected  ExpectedDTO     `json:"expected"`
	Actual    ActualDTO       `json:"actual"`
	Judge     JudgeDTO        `json:"judge"`
	Discovery DiscoveryDTO    `json:"discovery"`
	Phases    []PhaseDTO      `json:"phases"`
	Turns     []TurnDTO       `json:"turns"`
	Files     []FileChangeDTO `json:"files"`
	TestRuns  []TestRunDTO    `json:"test_runs"`
	Artifacts []ArtifactRef   `json:"artifacts"`
	Meta      map[string]any  `json:"meta"`
	Warnings  []string        `json:"warnings"`
}

// ExpectedDTO is what the fixture declared it wanted.
type ExpectedDTO struct {
	// Source is "snapshot" (this run's own record), "repo" (the source
	// tree as it stands now — may have drifted), or "absent".
	Source       string   `json:"source"`
	Command      string   `json:"command"`
	Answers      string   `json:"answers"`
	Prep         []string `json:"prep"`
	Teardown     []string `json:"teardown"`
	ExitCode     int      `json:"exit_code"`
	StdoutChecks []string `json:"stdout_checks"`
	JudgeSpec    string   `json:"judge_spec"`
	InputPath    string   `json:"input_path"`
}

// ActualDTO is what the run produced, paired against ExpectedDTO.
type ActualDTO struct {
	ExitCode  *int     `json:"exit_code"`
	Verdict   string   `json:"verdict"`
	Failures  []string `json:"failures"`
	DiffCount int      `json:"diff_count"`
	HasStdout bool     `json:"has_stdout"`
	HasStderr bool     `json:"has_stderr"`
}

// JudgeDTO is the harness's verdict call.
type JudgeDTO struct {
	Ran          bool           `json:"ran"`
	CLI          string         `json:"cli"`
	Model        string         `json:"model"`
	StartedAt    string         `json:"started_at"`
	EndedAt      string         `json:"ended_at"`
	CostUSD      float64        `json:"cost_usd"`
	Tokens       int            `json:"tokens"`
	TokensByKind map[string]int `json:"tokens_by_kind"`
	// HasTranscript is false for every session recorded before the
	// harness started persisting the judge prompt. The UI must say so
	// rather than render an empty diff, which would read as "identical".
	HasTranscript bool `json:"has_transcript"`
}

// DiscoveryDTO is the engine's test-run span.
type DiscoveryDTO struct {
	Found     bool   `json:"found"`
	Framework string `json:"framework"`
	Outcome   string `json:"outcome"`
}

// PhaseDTO is one timeline slice. It names its turn by index rather than
// embedding it — the loader's Phase holds a pointer back into the turn
// list, and following it in JSON would duplicate the whole turn.
type PhaseDTO struct {
	Label        string  `json:"label"`
	Detail       string  `json:"detail"`
	Kind         string  `json:"kind"`
	Owner        string  `json:"owner"`
	Role         string  `json:"role"`
	Seconds      float64 `json:"seconds"`
	Measured     bool    `json:"measured"`
	OffsetSecond float64 `json:"offset_seconds"`
	CostUSD      float64 `json:"cost_usd"`
	Tokens       int     `json:"tokens"`
	TurnIndex    *int    `json:"turn_index"`
}

// TurnDTO is one model call.
type TurnDTO struct {
	Index           int           `json:"index"`
	Number          int           `json:"number"`
	Role            string        `json:"role"`
	CLI             string        `json:"cli"`
	Model           string        `json:"model"`
	Status          string        `json:"status"`
	Error           string        `json:"error"`
	StartedAt       string        `json:"started_at"`
	EndedAt         string        `json:"ended_at"`
	DurationSeconds float64       `json:"duration_seconds"`
	DurationIsFloor bool          `json:"duration_is_floor"`
	CostUSD         float64       `json:"cost_usd"`
	HasCost         bool          `json:"has_cost"`
	Tokens          TokensDTO     `json:"tokens"`
	SystemLength    int           `json:"system_length"`
	UserLength      int           `json:"user_length"`
	AllowedTools    []string      `json:"allowed_tools"`
	DisallowedTools []string      `json:"disallowed_tools"`
	Cell            CellDTO       `json:"cell"`
	Command         string        `json:"command"`
	ToolCalls       []ToolCallDTO `json:"tool_calls"`
	Inputs          []ArtifactRef `json:"inputs"`
	Outputs         []ArtifactRef `json:"outputs"`
}

// TokensDTO keeps the four counters apart — cache reads and cache writes
// price differently from ordinary input.
type TokensDTO struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	CacheRead     int `json:"cache_read"`
	CacheCreation int `json:"cache_creation"`
	Total         int `json:"total"`
}

// CellDTO identifies which checklist cell a turn belongs to. Key is what
// turn alignment across runs is computed on.
type CellDTO struct {
	Index   string `json:"index"`
	Subject string `json:"subject"`
	Section string `json:"section"`
	Label   string `json:"label"`
	Key     string `json:"key"`
}

// ToolCallDTO is one tool the model invoked.
type ToolCallDTO struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

// ArtifactRef points at a body without carrying it. Ref is the opaque
// token the text endpoint accepts — never a filesystem path from the
// client, so there is no traversal surface.
type ArtifactRef struct {
	Ref  string `json:"ref"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	// RepoPath locates this artifact from the repo root, for a reader who
	// would rather open it than read it in the page.
	RepoPath string `json:"repo_path"`
	Bytes    int    `json:"bytes"`
	Missing  bool   `json:"missing"`
}

// FileChangeDTO is one entry of the run's diff.
type FileChangeDTO struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	// RepoPath is Path relative to the repo root, which is what a reader
	// can actually open. Path alone is relative to the run's tmpdir and
	// locates nothing.
	RepoPath    string `json:"repo_path"`
	BytesBefore int    `json:"bytes_before"`
	BytesAfter  int    `json:"bytes_after"`
}

// TestRunDTO is one framework-runner subprocess.
type TestRunDTO struct {
	Framework       string      `json:"framework"`
	Phase           string      `json:"phase"`
	Label           string      `json:"label"`
	Outcome         string      `json:"outcome"`
	ExitCode        int         `json:"exit_code"`
	HasExit         bool        `json:"has_exit"`
	DurationSeconds float64     `json:"duration_seconds"`
	At              string      `json:"at"`
	Error           string      `json:"error"`
	Stdout          ArtifactRef `json:"stdout"`
	Stderr          ArtifactRef `json:"stderr"`
}
