package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// cellID identifies one evaluated cell within a partition.
type cellID struct {
	Index   int
	Subject string
}

// cellFiles holds the artifact paths discovered for one cell.
type cellFiles struct {
	SanitizedPath string
	PathConflict  bool
	ResponsePath  string
	ResultPath    string
	IterUserPath  string
	HasPromptsMD  bool
}

// runContext bundles everything needed to interpret one fixture run.
type runContext struct {
	run        FixtureRun
	universe   *Universe
	prompts    []UniversePrompt        // the run's own loaded checklist, flattened
	shipped    map[int]*UniversePrompt // run prompt index -> shipped prompt (nil = non-shipped)
	segments   []logSegment
	logTainted bool // malformed log lines seen: chronological proofs unusable
}

// CollectRun turns one fixture working directory into observations and
// diagnostics.
func CollectRun(run FixtureRun, uni *Universe, fixturesDir string) ([]Observation, []Diagnostic) {
	segments, logTainted, logDiag := loadSegments(run)

	diagnostics := make([]Diagnostic, 0)
	if logDiag != nil {
		diagnostics = append(diagnostics, *logDiag)
	}

	stem := stemFromLog(segments)

	if stem == "" {
		if partitionsHaveFiles(run) {
			return nil, append(diagnostics, Diagnostic{Fixture: run.Fixture, Hard: true,
				Message: "artifacts present but no command evidence in the run log"})
		}

		if !fixtureExists(fixturesDir, run.Fixture) && len(segments) == 0 {
			return nil, append(diagnostics, Diagnostic{Fixture: run.Fixture, Hard: false,
				Message: "no command evidence in the run log — run skipped, not identifiable as a help run"})
		}

		// Segments without a stem still carry log records (parseLogSpine
		// emits one for "Result file saved" even without "Loaded prompts");
		// silently dropping this would hide a run the tool cannot name.
		if len(segments) > 0 {
			return nil, append(diagnostics, Diagnostic{Fixture: run.Fixture, Hard: false,
				Message: fmt.Sprintf(
					"%d log segment(s) but no command stem — the run is not identifiable as a help run",
					len(segments))})
		}

		return nil, diagnostics // no checklist command (help-flag): zero observations
	}

	ctx, diag := buildRunContext(run, uni, stem, segments)
	if diag != nil {
		return nil, append(diagnostics, *diag)
	}

	ctx.logTainted = logTainted

	observations := make([]Observation, 0)

	for _, partition := range run.Partitions {
		obs, diags := collectPartition(ctx, partition)

		observations = append(observations, obs...)
		diagnostics = append(diagnostics, diags...)
	}

	return observations, diagnostics
}

// loadSegments parses the run's log when present; a missing log means
// artifact-only collection, but an unreadable or partially malformed
// one stays visible as a diagnostic (fail-closed evidence policy).
func loadSegments(run FixtureRun) ([]logSegment, bool, *Diagnostic) {
	if run.LogPath == "" {
		return nil, false, nil
	}

	segments, malformed, err := parseLogSpine(run.LogPath)
	if err != nil {
		return nil, false, &Diagnostic{Fixture: run.Fixture, Hard: false,
			Message: "log unreadable, degraded to artifact-only: " + err.Error()}
	}

	if malformed > 0 {
		return segments, true, &Diagnostic{Fixture: run.Fixture, Hard: false,
			Message: fmt.Sprintf("log contains %d malformed line(s) — "+
				"chronological proofs disabled for this run", malformed)}
	}

	return segments, false, nil
}

// fixtureExists reports whether the run names a fixture that is still in
// the source tree. It used to ask whether the fixture's manifest existed;
// a fixture is a directory now, so the directory is the question.
func fixtureExists(fixturesDir, fixtureName string) bool {
	info, err := os.Stat(filepath.Join(fixturesDir, fixtureName))

	return err == nil && info.IsDir()
}

// partitionsHaveFiles reports whether any partition contains entries.
func partitionsHaveFiles(run FixtureRun) bool {
	for _, partition := range run.Partitions {
		entries, err := os.ReadDir(partition)
		if err == nil && len(entries) > 0 {
			return true
		}
	}

	return false
}

// buildRunContext loads the run's own checklist copy and maps each of
// its prompts to the shipped universe by eval-field equality.
func buildRunContext(
	run FixtureRun, uni *Universe, stem string, segments []logSegment,
) (*runContext, *Diagnostic) {
	if run.ConfigDir == "" {
		return nil, &Diagnostic{Fixture: run.Fixture, Hard: true,
			Message: "no engine config dir found in run dir"}
	}

	path := filepath.Join(run.ConfigDir, "checklists", stem+".yaml")

	prompts, err := loadChecklistPrompts(path, stem)
	if err != nil {
		return nil, &Diagnostic{Fixture: run.Fixture, Hard: true,
			Message: fmt.Sprintf("cannot load run checklist: %v", err)}
	}

	shipped := make(map[int]*UniversePrompt, len(prompts))
	for idx := range prompts {
		shipped[prompts[idx].Global] = uni.MatchOverride(stem, prompts[idx])
	}

	return &runContext{
		run: run, universe: uni, prompts: prompts, shipped: shipped, segments: segments,
	}, nil
}

// segmentFor finds the unique log segment bound to a partition; the
// bool reports an ambiguity (two invocations sharing one minute dir).
func (c *runContext) segmentFor(partitionBase string) (*logSegment, bool) {
	var found *logSegment

	for segIdx := range c.segments {
		if c.segments[segIdx].Partition != partitionBase {
			continue
		}

		if found != nil {
			return nil, true
		}

		found = &c.segments[segIdx]
	}

	return found, false
}

// collectPartition extracts every observation from one tmp/<ts> dir.
func collectPartition(ctx *runContext, partition string) ([]Observation, []Diagnostic) {
	base := filepath.Base(partition)

	segment, ambiguous := ctx.segmentFor(base)
	if ambiguous {
		return nil, []Diagnostic{{Fixture: ctx.run.Fixture, Hard: true,
			Message: "partition " + base + " matches multiple log segments (same-minute collision)"}}
	}

	cells, applies, err := scanPartitionFiles(ctx, partition)
	if err != nil {
		return nil, []Diagnostic{{Fixture: ctx.run.Fixture, Hard: true, Message: err.Error()}}
	}

	state := newPartitionState(ctx, partition, base, segment, cells, applies)

	state.classifyCells()
	state.attributeAndChainApplies()

	return state.observations, state.diagnostics
}

// scanPartitionFiles decodes every artifact filename in the partition.
func scanPartitionFiles(
	ctx *runContext, partition string,
) (map[cellID]*cellFiles, []ApplyArtifact, error) {
	entries, err := os.ReadDir(partition)
	if err != nil {
		return nil, nil, fmt.Errorf("reading partition %s: %w", partition, err)
	}

	paths := sanitizedPathsOf(ctx.prompts)
	cells := map[cellID]*cellFiles{}
	applies := make([]ApplyArtifact, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		fullPath := filepath.Join(partition, name)

		if ev, ok := decodeEvaluatorName(name, paths); ok {
			cell := ensureCell(cells, cellID{ev.PromptIndex, ev.Subject})
			applyEvaluatorFile(cell, ev, name, fullPath)

			continue
		}

		if fix, ok := decodeFixName(name); ok {
			cell := ensureCell(cells, cellID{fix.PromptIndex, fix.Subject})
			applyFixFile(cell, fix, fullPath)

			continue
		}

		if apply, ok := decodeApplyName(name); ok && apply.Kind == kindResultYAML {
			applies = append(applies, apply)
		}
	}

	return cells, applies, nil
}

// sanitizedPathsOf returns the unique sanitized section paths of the
// run checklist, used to right-anchor evaluator filenames.
func sanitizedPathsOf(prompts []UniversePrompt) []string {
	seen := map[string]bool{}
	paths := make([]string, 0)

	for _, p := range prompts {
		if !seen[p.SanitizedPath] {
			seen[p.SanitizedPath] = true

			paths = append(paths, p.SanitizedPath)
		}
	}

	return paths
}

// ensureCell fetches or creates the record for one cell.
func ensureCell(cells map[cellID]*cellFiles, id cellID) *cellFiles {
	if cells[id] == nil {
		cells[id] = &cellFiles{}
	}

	return cells[id]
}

// applyEvaluatorFile records one evaluator artifact on its cell,
// flagging a conflict when two different section paths claim one cell.
func applyEvaluatorFile(cell *cellFiles, artifact EvaluatorArtifact, name, fullPath string) {
	for _, p := range knownSanitizedSuffixes(name, artifact) {
		if cell.SanitizedPath != "" && cell.SanitizedPath != p {
			cell.PathConflict = true
		}

		cell.SanitizedPath = p
	}

	switch artifact.Kind {
	case kindResponse:
		cell.ResponsePath = fullPath
	case kindResultYAML:
		cell.ResultPath = fullPath
	}
}

// knownSanitizedSuffixes re-derives which sanitized path the evaluator
// filename carried (decodeEvaluatorName already matched exactly one).
func knownSanitizedSuffixes(name string, artifact EvaluatorArtifact) []string {
	prefixLen := len(name) - len("-"+artifact.Kind)
	marker := "-checklist-"

	idx := lastIndex(name[:prefixLen], marker)
	if idx == -1 {
		return nil
	}

	return []string{name[idx+len(marker) : prefixLen]}
}

// lastIndex is strings.LastIndex without importing strings twice in
// this file's hot path helpers.
func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}

// applyFixFile records one fix-generation artifact on its cell.
func applyFixFile(cell *cellFiles, fix FixArtifact, fullPath string) {
	switch fix.Kind {
	case kindPromptsMD:
		cell.HasPromptsMD = true
	case kindUser:
		if cell.IterUserPath == "" || fullPath < cell.IterUserPath {
			cell.IterUserPath = fullPath
		}
	}
}
