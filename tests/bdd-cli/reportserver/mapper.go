package reportserver

import (
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/reporter"
	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/runner"
)

// stamp renders a time for the wire, or "" for the zero value. A zero
// time must not cross as "0001-01-01…" — the client would render it as a
// real instant in the year 1.
func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}

	return at.Format(time.RFC3339Nano)
}

// mapRunSummary projects a run's roll-up.
func mapRunSummary(run *Run) RunSummary {
	return RunSummary{
		ID:           run.ID,
		Complete:     run.Complete,
		Fixtures:     run.Totals.Fixtures,
		Passed:       run.Totals.Passed,
		Failed:       run.Totals.Failed,
		Skipped:      run.Totals.Skipped,
		Turns:        run.Totals.Turns,
		Tokens:       run.Totals.Tokens,
		WallSeconds:  run.Totals.Wall.Seconds(),
		CostUSD:      run.Totals.CostUSD,
		JudgeCostUSD: run.Totals.JudgeCostUSD,
	}
}

// mapTestSummary projects one fixture row.
func mapTestSummary(fixture *reporter.Fixture) TestSummary {
	summary := TestSummary{
		Name:              fixture.Name,
		Verdict:           fixture.Verdict,
		Command:           fixture.Command,
		HasRecord:         fixture.HasRecord,
		WallSeconds:       fixture.Wall.Seconds(),
		HasWall:           fixture.HasWall,
		PhaseTotalSeconds: fixture.PhaseTotal,
		Turns:             len(fixture.Turns),
		ModelSeconds:      fixture.ModelSeconds,
		CostUSD:           fixture.Cost,
		Tokens:            fixture.Tokens,
		DiffCount:         len(fixture.Diff),
		Failures:          orEmpty(fixture.Failures),
		ManifestSource:    string(fixture.Manifest.Source),
	}

	// Drift is the timeline's own self-check: how far the accounted
	// phases land from the wall clock the harness measured. Reported per
	// test rather than logged, because a number that is not near zero
	// means the phase model has a hole.
	if fixture.HasWall {
		summary.DriftSeconds = fixture.Wall.Seconds() - fixture.PhaseTotal
	}

	if fixture.Judge != nil {
		summary.JudgeCostUSD = fixture.Judge.CostUSD
	}

	if fixture.Record != nil {
		summary.ExitCode = fixture.Record.ExitCode
		summary.DiffCount = len(fixture.Record.Diff)
	}

	return summary
}

// mapExpected projects the fixture's declared expectations.
func mapExpected(manifest *reporter.Manifest) ExpectedDTO {
	source := manifest.Source
	if source == "" {
		source = reporter.ManifestAbsent
	}

	return ExpectedDTO{
		Source:       string(source),
		Command:      manifest.Command,
		Answers:      manifest.Answers,
		Prep:         orEmpty(manifest.PrepCmds),
		Teardown:     orEmpty(manifest.TeardownCmds),
		ExitCode:     manifest.ExpectedExit,
		StdoutChecks: orEmpty(manifest.StdoutChecks),
		JudgeSpec:    manifest.JudgeSpec,
		InputPath:    manifest.InputPath,
	}
}

// mapActual projects what the run produced.
func mapActual(fixture *reporter.Fixture) ActualDTO {
	actual := ActualDTO{
		Verdict:   fixture.Verdict,
		Failures:  orEmpty(fixture.Failures),
		DiffCount: len(fixture.Diff),
	}

	if fixture.Record != nil {
		actual.ExitCode = fixture.Record.ExitCode
		actual.DiffCount = len(fixture.Record.Diff)
		actual.HasStdout = fixture.Record.StdoutFile != ""
		actual.HasStderr = fixture.Record.StderrFile != ""
	}

	return actual
}

// mapJudge projects the verdict call, including whether its transcript
// was captured at all.
func mapJudge(fixture *reporter.Fixture) JudgeDTO {
	judge := JudgeDTO{}

	if fixture.Record != nil {
		record := fixture.Record.Judge
		judge.Ran = !record.EndedAt.IsZero()
		judge.CLI = record.CLI
		judge.Model = record.Model
		judge.StartedAt = stamp(record.StartedAt)
		judge.EndedAt = stamp(record.EndedAt)
		judge.CostUSD = record.CostUSD
		judge.Tokens = record.Tokens
		judge.TokensByKind = record.TokensByKind
		judge.HasTranscript = hasArtifact(fixture.Record, runner.JudgeUserFile)
	}

	if fixture.Judge != nil && !judge.Ran {
		judge.Ran = true
		judge.CostUSD = fixture.Judge.CostUSD
		judge.Tokens = fixture.Judge.Tokens
		judge.EndedAt = stamp(fixture.Judge.At)
	}

	return judge
}

// hasArtifact reports whether the run advertised a sidecar by name.
func hasArtifact(record *runner.HarnessRecord, name string) bool {
	for _, got := range record.Artifacts {
		if got == name {
			return true
		}
	}

	return false
}

// mapPhases projects the timeline, replacing each slice's turn pointer
// with that turn's index in the fixture's turn list.
func mapPhases(fixture *reporter.Fixture) []PhaseDTO {
	index := make(map[*reporter.Turn]int, len(fixture.Turns))
	for position, turn := range fixture.Turns {
		index[turn] = position
	}

	phases := make([]PhaseDTO, 0, len(fixture.Phases))

	for _, phase := range fixture.Phases {
		dto := PhaseDTO{
			Label:        phase.Label,
			Detail:       phase.Detail,
			Kind:         string(phase.Kind),
			Owner:        string(phase.Owner),
			Role:         phase.Role,
			Seconds:      phase.Seconds,
			Measured:     phase.Measured,
			OffsetSecond: phase.Offset,
			CostUSD:      phase.CostUSD,
			Tokens:       phase.Tokens,
		}

		if phase.Turn != nil {
			if position, ok := index[phase.Turn]; ok {
				dto.TurnIndex = &position
			}
		}

		phases = append(phases, dto)
	}

	return phases
}

// mapTurn projects one model call.
func mapTurn(position int, turn *reporter.Turn) TurnDTO {
	return TurnDTO{
		Index:           position,
		Number:          turn.Number,
		Role:            turn.Role,
		CLI:             turn.CLI,
		Model:           turn.Model,
		Status:          string(turn.Status),
		Error:           turn.Error,
		StartedAt:       stamp(turn.Started),
		EndedAt:         stamp(turn.Ended),
		DurationSeconds: turn.Duration.Seconds(),
		DurationIsFloor: turn.DurationIsFloor,
		CostUSD:         turn.CostUSD,
		HasCost:         turn.HasCost,
		Tokens: TokensDTO{
			Input:         turn.Tokens.Input,
			Output:        turn.Tokens.Output,
			CacheRead:     turn.Tokens.CacheRead,
			CacheCreation: turn.Tokens.CacheCreation,
			Total:         turn.Tokens.Total(),
		},
		SystemLength:    turn.SystemLength,
		UserLength:      turn.UserLength,
		AllowedTools:    orEmpty(turn.AllowedTools),
		DisallowedTools: orEmpty(turn.DisallowedTools),
		Cell:            mapCell(turn),
		Command:         turn.Invocation.Command(),
		ToolCalls:       mapToolCalls(turn.ToolCalls),
		Inputs:          mapArtifacts(position, "in", turn.Inputs),
		Outputs:         mapArtifacts(position, "out", turn.Outputs),
	}
}

// mapCell projects a turn's checklist cell and the key turn alignment
// uses.
func mapCell(turn *reporter.Turn) CellDTO {
	return CellDTO{
		Index:   turn.Cell.Index,
		Subject: turn.Cell.Subject,
		Section: turn.Cell.Section,
		Label:   turn.Cell.String(),
		Key:     turnKey(turn),
	}
}

// mapToolCalls projects the tools a turn invoked.
func mapToolCalls(calls []reporter.ToolCall) []ToolCallDTO {
	out := make([]ToolCallDTO, 0, len(calls))
	for _, call := range calls {
		out = append(out, ToolCallDTO{Name: call.Name, Input: call.Input})
	}

	return out
}

// mapArtifacts projects a turn's artifacts as references. The body stays
// on the server until something asks for it by ref.
func mapArtifacts(turnIndex int, side string, artifacts []reporter.Artifact) []ArtifactRef {
	refs := make([]ArtifactRef, 0, len(artifacts))

	for position, artifact := range artifacts {
		refs = append(refs, ArtifactRef{
			Ref:     "turn:" + strconv.Itoa(turnIndex) + ":" + side + ":" + strconv.Itoa(position),
			Kind:    artifact.Kind,
			Name:    artifact.Name,
			Bytes:   artifact.Bytes,
			Missing: artifact.Missing,
		})
	}

	return refs
}

// mapTestRuns projects the framework-runner subprocesses.
func mapTestRuns(runs []reporter.TestRun) []TestRunDTO {
	out := make([]TestRunDTO, 0, len(runs))

	for position, run := range runs {
		out = append(out, TestRunDTO{
			Framework:       run.Framework,
			Phase:           run.Phase,
			Label:           run.Label(),
			Outcome:         run.Outcome(),
			ExitCode:        run.ExitCode,
			HasExit:         run.HasExit,
			DurationSeconds: run.Duration.Seconds(),
			At:              stamp(run.At),
			Error:           run.Error,
			Stdout:          streamRef(position, "stdout", run.Stdout),
			Stderr:          streamRef(position, "stderr", run.Stderr),
		})
	}

	return out
}

// streamRef references one captured test-runner stream.
func streamRef(index int, stream string, artifact reporter.Artifact) ArtifactRef {
	return ArtifactRef{
		Ref:     "testrun:" + strconv.Itoa(index) + ":" + stream,
		Kind:    stream,
		Name:    artifact.Name,
		Bytes:   artifact.Bytes,
		Missing: artifact.Missing,
	}
}

// mapFiles projects the run's structural diff. Falls back to the
// rendered lines for a session recorded before the structural form
// existed, so an old run still lists what it changed.
func mapFiles(fixture *reporter.Fixture) []FileChangeDTO {
	if fixture.Record == nil {
		return legacyFiles(fixture.Diff)
	}

	files := make([]FileChangeDTO, 0, len(fixture.Record.Diff))
	for _, change := range fixture.Record.Diff {
		files = append(files, FileChangeDTO{
			Kind:        change.Kind,
			Path:        change.Path,
			BytesBefore: change.BytesBefore,
			BytesAfter:  change.BytesAfter,
		})
	}

	return files
}

// legacyFiles parses "created path (N bytes)" back into structure.
func legacyFiles(lines []string) []FileChangeDTO {
	files := make([]FileChangeDTO, 0, len(lines))

	for _, line := range lines {
		kind, rest, found := strings.Cut(line, " ")
		if !found {
			continue
		}

		path, _, _ := strings.Cut(rest, " (")
		files = append(files, FileChangeDTO{Kind: kind, Path: path})
	}

	return files
}

// mapMeta projects the engine's run metadata as a flat bag — it is
// display-only, and a typed DTO per field would grow with the engine.
func mapMeta(meta reporter.RunMetadata) map[string]any {
	return map[string]any{
		"architecture_file":  meta.ArchitectureFile,
		"services":           meta.Services,
		"checklist_command":  meta.ChecklistCommand,
		"checklist_path":     meta.ChecklistPath,
		"items":              meta.Items,
		"prompts":            meta.Prompts,
		"max_apply_attempts": meta.MaxApplyAttempts,
		"test_run_command":   meta.TestRun.Command(),
		"decisions":          len(meta.Decisions),
	}
}

// orEmpty turns a nil slice into an empty one, so the wire carries [] and
// not null — a client should not have to guard every list.
func orEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}

	return items
}
