package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Engine log messages the report folds into turns. Everything else in
// the log is DEBUG noise from the provider's message stream.
const (
	msgDispatch  = "Dispatching AI turn"
	msgAssistant = "AssistantMessage received"
	msgResult    = "ResultMessage received"
	// msgTranscriptSaved is crush's and codex's only result boundary:
	// they stream nothing, so the archived transcript is the first
	// evidence the turn produced anything.
	msgTranscriptSaved = "CLI transcript saved"
	msgUsage           = "AI turn usage"
	msgReturned        = "AI turn returned"
	msgFailed          = "AI turn failed"
)

// Engine logs can carry a single very long line — a rendered prompt or a
// tool-use blob — so the scanner is given room well past bufio's 64KB
// default before it would call a line too long.
const (
	initialScanBuffer = 64 * 1024
	maxScanTokenSize  = 8 * 1024 * 1024
)

// tokenKeys are the four counters the claude CLI reports. Cache reads
// and cache writes are priced differently from ordinary input, so the
// engine logs them separately; the report only ever needs the sum.
func tokenKeys() []string {
	return []string{
		"input_tokens",
		"output_tokens",
		"cache_read_input_tokens",
		"cache_creation_input_tokens",
	}
}

// LogRecord is one line of the engine's JSON log, decoded to just the
// fields the report reads. Unknown fields are ignored.
type LogRecord struct {
	Time       string   `json:"time"`
	Level      string   `json:"level"`
	Msg        string   `json:"msg"`
	Turn       *int     `json:"turn"`
	Role       string   `json:"role"`
	CLI        string   `json:"cli"`
	Model      string   `json:"model"`
	DurationMs *int64   `json:"duration_ms"`
	CostUSD    *float64 `json:"cost_usd"`
	Error      string   `json:"error"`

	SystemLength int `json:"system_length"`
	UserLength   int `json:"user_length"`

	// File is the artifact path on the "… saved" records.
	File string `json:"file"`
	// Content carries a tool-use block's JSON payload.
	Content string `json:"content"`

	AllowedTools    []string `json:"allowed_tools"`
	DisallowedTools []string `json:"disallowed_tools"`

	// Binary/Args/Dir describe a spawned subprocess.
	Binary string   `json:"binary"`
	Args   []string `json:"args"`
	Dir    string   `json:"dir"`

	// Framework/Phase and the Exit* fields describe a test-runner
	// subprocess. They arrive on "Test runner returned", which is
	// written AFTER the process exits — unlike the spawn record, which
	// proves only that the engine intended to run something. DurationMs
	// above is reused for the measured wall clock.
	Framework   string `json:"framework"`
	Phase       string `json:"phase"`
	ExitCode    *int   `json:"exit_code"`
	StdoutBytes *int   `json:"stdout_bytes"`
	StderrBytes *int   `json:"stderr_bytes"`
	StdoutFile  string `json:"stdout_file"`
	StderrFile  string `json:"stderr_file"`

	// Fields carried by the engine's own progress records.
	Command          string `json:"command"`
	Path             string `json:"path"`
	Items            *int   `json:"items"`
	Prompts          *int   `json:"prompts"`
	MaxApplyAttempts *int   `json:"max_apply_attempts"`
	Services         *int   `json:"services"`
	SubjectID        string `json:"subjectID"` //nolint:tagliatelle // slog field name is the wire contract
	Section          string `json:"section"`
	Iteration        *int   `json:"iteration"`
	Action           string `json:"action"`
	RawInput         string `json:"rawInput"` //nolint:tagliatelle // slog field name is the wire contract

	// At is Time parsed once, offset as written.
	At time.Time `json:"-"`
	// Tokens holds whichever of tokenKeys the record carried.
	Tokens map[string]int `json:"-"`
}

// EngineLog is one fixture's parsed engine log.
type EngineLog struct {
	Records []LogRecord
	First   time.Time
	Last    time.Time
}

// LoadEngineLog reads a true-bdd.log.json. Unparsable lines are skipped
// rather than fatal: the log is append-only and a run killed mid-write
// can leave a torn final line, which should not cost the whole report.
func LoadEngineLog(path string) (*EngineLog, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open engine log: %w", err)
	}
	defer func() { _ = file.Close() }()

	log := &EngineLog{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, initialScanBuffer), maxScanTokenSize)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}

		record, ok := decodeRecord(line)
		if !ok {
			continue
		}

		if !record.At.IsZero() {
			if log.First.IsZero() {
				log.First = record.At
			}

			log.Last = record.At
		}

		log.Records = append(log.Records, record)
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("read engine log: %w", err)
	}

	return log, nil
}

// decodeRecord parses one JSON line, pulling the token counters out of
// the flat field set the engine logs them in.
func decodeRecord(line string) (LogRecord, bool) {
	var record LogRecord

	err := json.Unmarshal([]byte(line), &record)
	if err != nil {
		return LogRecord{}, false
	}

	record.At = parseLogTime(record.Time)

	var raw map[string]json.RawMessage

	err = json.Unmarshal([]byte(line), &raw)
	if err != nil {
		return record, true
	}

	for _, key := range tokenKeys() {
		blob, present := raw[key]
		if !present {
			continue
		}

		var count int

		err = json.Unmarshal(blob, &count)
		if err != nil {
			continue
		}

		if record.Tokens == nil {
			record.Tokens = map[string]int{}
		}

		record.Tokens[key] = count
	}

	return record, true
}

// parseLogTime reads slog's RFC3339 stamp, offset and all. The zone is
// left as written: every use is a subtraction between two instants, and
// those are absolute regardless of which offset each side carries. A
// zero time means "unparsable", never "epoch".
func parseLogTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

// turnFolder walks the record stream once, attaching everything a turn
// produced to that turn.
//
// Artifacts are matched positionally, which the log supports exactly:
// prompt artifacts are written immediately before the dispatch that
// consumes them, and responses immediately after the completion that
// produced them. Kind decides which side of a boundary a file falls on —
// a system or user prompt in the gap between two turns belongs to the
// NEXT turn, anything else to the one that just closed.
type turnFolder struct {
	fixtureDir string
	turns      []*Turn
	open       *Turn
	lastClosed *Turn
	// pendingInputs are prompt artifacts seen since the last turn ended,
	// waiting for the dispatch that will claim them.
	pendingInputs []Artifact
	// pendingTools are the permissions logged just before a dispatch.
	pendingAllowed    []string
	pendingDisallowed []string
	pendingSpawn      Invocation
}

// Turns folds the record stream into one Turn per dispatch, in order.
func (l *EngineLog) Turns(fixtureDir string) []*Turn {
	folder := &turnFolder{fixtureDir: fixtureDir}

	for index := range l.Records {
		folder.consume(&l.Records[index])
	}

	l.closeUnfinished(folder.turns)
	inheritCellSections(folder.turns)

	return folder.turns
}

// consume folds one record into the run being reconstructed.
func (f *turnFolder) consume(record *LogRecord) {
	switch {
	case record.Msg == msgDispatch:
		f.startTurn(record)
	case isArtifactRecord(record.Msg) && record.File != "":
		f.attachArtifact(record)
	case record.Msg == msgToolsConfigured:
		f.rememberTools(record)
	case record.Msg == msgSpawnAgent:
		f.attachSpawn(record)
	case f.open != nil:
		f.applyToOpenTurn(record)
	}
}

// attachSpawn records the command a turn's subprocess was started with.
//
// The spawn record lands just AFTER the dispatch, not before it: the
// router logs the dispatch, then calls the provider, which builds the
// argv and spawns. So it belongs to the turn already open — holding it
// for the next dispatch would shift every command one turn late, which
// is exactly wrong when consecutive turns run on different CLIs.
func (f *turnFolder) attachSpawn(record *LogRecord) {
	invocation := Invocation{
		Binary: record.Binary, Args: record.Args, Dir: record.Dir, Known: true,
	}

	if f.open != nil {
		f.open.Invocation = invocation

		return
	}

	f.pendingSpawn = invocation
}

// startTurn opens a turn and hands it everything gathered on its behalf
// before the dispatch record was written.
func (f *turnFolder) startTurn(record *LogRecord) {
	turn := &Turn{
		Role:            record.Role,
		CLI:             record.CLI,
		Model:           record.Model,
		Started:         record.At,
		Status:          TurnOpen,
		SystemLength:    record.SystemLength,
		UserLength:      record.UserLength,
		Inputs:          f.pendingInputs,
		AllowedTools:    f.pendingAllowed,
		DisallowedTools: f.pendingDisallowed,
		Invocation:      f.pendingSpawn,
	}

	if record.Turn != nil {
		turn.Number = *record.Turn
	}

	for _, artifact := range turn.Inputs {
		if cell, ok := cellFromArtifact(artifact.Name); ok {
			turn.Cell = cell

			break
		}
	}

	f.turns = append(f.turns, turn)
	f.open = turn
	f.pendingInputs = nil
	f.pendingAllowed = nil
	f.pendingDisallowed = nil
	f.pendingSpawn = Invocation{}
}

// attachArtifact files one written artifact against the turn it belongs
// to, or holds it for the turn about to start.
func (f *turnFolder) attachArtifact(record *LogRecord) {
	artifact := loadArtifact(f.fixtureDir, record.File)

	if artifact.Kind == "system prompt" || artifact.Kind == "user prompt" {
		f.pendingInputs = append(f.pendingInputs, artifact)

		return
	}

	target := f.open
	if target == nil {
		target = f.lastClosed
	}

	if target == nil {
		return
	}

	// The archived transcript is the only completion signal crush and
	// codex give: they stream no messages, so without this their turns
	// would have no result boundary and would look unsplittable AND
	// unfinished.
	if artifact.Kind == "CLI transcript" && target.ResultAt.IsZero() {
		target.ResultAt = record.At
	}

	target.Outputs = append(target.Outputs, artifact)
}

// rememberTools records the permissions a turn ran under. Like the
// spawn record, the provider logs these just after the dispatch, so they
// belong to the turn already open; the pending fields are the fallback
// for a log whose ordering differs. The allow and deny lists arrive as
// two separate records.
func (f *turnFolder) rememberTools(record *LogRecord) {
	if f.open != nil {
		if len(record.AllowedTools) > 0 {
			f.open.AllowedTools = record.AllowedTools
		}

		if len(record.DisallowedTools) > 0 {
			f.open.DisallowedTools = record.DisallowedTools
		}

		return
	}

	if len(record.AllowedTools) > 0 {
		f.pendingAllowed = record.AllowedTools
	}

	if len(record.DisallowedTools) > 0 {
		f.pendingDisallowed = record.DisallowedTools
	}
}

// applyToOpenTurn folds one record into the turn currently in flight.
func (f *turnFolder) applyToOpenTurn(record *LogRecord) {
	switch record.Msg {
	case msgAssistant:
		if f.open.FirstOutput.IsZero() {
			f.open.FirstOutput = record.At
		}
	case msgResult:
		if f.open.ResultAt.IsZero() {
			f.open.ResultAt = record.At
		}
	case msgToolUse:
		if call, ok := parseToolCall(record.Content); ok {
			f.open.ToolCalls = append(f.open.ToolCalls, call)
		}
	case msgUsage:
		applyUsage(f.open, record)
	case msgReturned, msgFailed:
		f.closeOpenTurn(record)
	}
}

// closeOpenTurn stamps the completion record onto the right turn and
// makes it the one that later artifacts attach to.
func (f *turnFolder) closeOpenTurn(record *LogRecord) {
	target := f.open

	if record.Turn != nil {
		if found := findTurn(f.turns, *record.Turn); found != nil {
			target = found
		}
	}

	closeTurn(target, record)

	f.lastClosed = target
	f.open = nil
}

// applyUsage records what the CLI reported it spent on the open turn.
func applyUsage(open *Turn, record *LogRecord) {
	if record.CostUSD != nil {
		open.CostUSD = *record.CostUSD
		open.HasCost = true
	}

	for _, key := range tokenKeys() {
		open.Tokens.Add(key, record.Tokens[key])
	}
}

// closeTurn stamps a turn with its completion record.
func closeTurn(turn *Turn, record *LogRecord) {
	if record.DurationMs != nil {
		turn.Duration = time.Duration(*record.DurationMs) * time.Millisecond
	}

	turn.Ended = record.At
	turn.Error = record.Error

	turn.Status = TurnOK
	if record.Msg == msgFailed {
		turn.Status = TurnFailed
	}
}

// findTurn locates a turn by its engine-assigned number.
func findTurn(turns []*Turn, number int) *Turn {
	for _, turn := range turns {
		if turn.Number == number {
			return turn
		}
	}

	return nil
}

// closeUnfinished gives any turn still open at EOF a lower-bound
// duration from the log's final record. The real figure is unknowable —
// the process stopped writing — so it is marked as a floor and the
// report renders it with a "≥".
func (l *EngineLog) closeUnfinished(turns []*Turn) {
	for _, turn := range turns {
		if turn.Status != TurnOpen || turn.Started.IsZero() || l.Last.IsZero() {
			continue
		}

		elapsed := l.Last.Sub(turn.Started)
		if elapsed < 0 {
			elapsed = 0
		}

		turn.Duration = elapsed
		turn.DurationIsFloor = true
		turn.Ended = l.Last
	}
}
