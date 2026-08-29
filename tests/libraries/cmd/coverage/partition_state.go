package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// partitionState accumulates observations while interpreting one
// partition's cells, applies, and log segment.
type partitionState struct {
	ctx          *runContext
	partition    string
	base         string
	segment      *logSegment
	cells        map[cellID]*cellFiles
	applies      []ApplyArtifact
	finalClass   map[cellID]VerdictClass
	failedCells  map[cellID]bool // cells with a semantic fail observation
	observations []Observation
	diagnostics  []Diagnostic
}

// newPartitionState wires the accumulator.
func newPartitionState(
	ctx *runContext, partition, base string, segment *logSegment,
	cells map[cellID]*cellFiles, applies []ApplyArtifact,
) *partitionState {
	return &partitionState{
		ctx: ctx, partition: partition, base: base, segment: segment,
		cells: cells, applies: applies,
		finalClass:  map[cellID]VerdictClass{},
		failedCells: map[cellID]bool{},
	}
}

// observe appends one observation for a cell.
func (s *partitionState) observe(cell cellID, kind ObservationKind, evidence, note string) {
	s.observations = append(s.observations, Observation{
		Fixture:     s.ctx.run.Fixture,
		Session:     s.ctx.run.Session,
		Partition:   s.base,
		Subject:     cell.Subject,
		PromptIndex: cell.Index,
		Kind:        kind,
		Prompt:      s.ctx.shipped[cell.Index],
		Evidence:    evidence,
		Note:        note,
	})
}

// diagnose appends one soft diagnostic (hard ones are constructed at
// the collection level).
func (s *partitionState) diagnose(message string) {
	s.diagnostics = append(s.diagnostics,
		Diagnostic{Fixture: s.ctx.run.Fixture, Hard: false, Message: message})
}

// classifyCells derives final verdicts from responses and pre-fix
// verdicts from fix-iteration artifacts. Non-shipped prompts are
// bucketed at merge time (credit-kind observation with nil Prompt).
func (s *partitionState) classifyCells() {
	for cell, files := range s.cells {
		prompt := s.promptAt(cell.Index)
		if prompt == nil {
			s.diagnose(fmt.Sprintf(
				"artifact references prompt index %d outside the run checklist", cell.Index))

			continue
		}

		if files.PathConflict || (files.SanitizedPath != "" &&
			files.SanitizedPath != prompt.SanitizedPath) {
			s.diagnose(fmt.Sprintf(
				"section path %q disagrees with prompt %d (%q) — cell skipped, uncredited",
				files.SanitizedPath, cell.Index, prompt.SanitizedPath))

			continue
		}

		s.classifyFinal(cell, files)
		s.classifyPreFix(cell, files)
	}
}

// classifyFinal classifies the surviving response for a cell.
func (s *partitionState) classifyFinal(cell cellID, files *cellFiles) {
	if files.ResponsePath == "" {
		if files.ResultPath != "" {
			s.diagnose("result without response for " + files.ResultPath + " — stale, uncredited")
		}

		return
	}

	data, err := disk.Read(files.ResponsePath)
	if err != nil {
		s.diagnose("unreadable response " + files.ResponsePath)

		return
	}

	if s.warnAfterLastSave(cell, files.SanitizedPath) {
		s.observe(cell, ObsFailProtocol, files.ResponsePath,
			"log records a protocol WARN after the last result save — surviving response is stale")
		s.diagnose("stale response suspected for " + files.ResponsePath)

		return
	}

	expected := "tmp/" + s.base + "/" + resultFileName(cell, files.SanitizedPath)
	class := ClassifyResponse(string(data), expected)

	if s.resultConflicts(files, class) {
		s.observe(cell, ObsFailUnclassed, files.ResponsePath,
			"surviving response and result.yaml carry conflicting canonical verdicts — "+
				"stale response suspected, cell uncredited")
		s.diagnose("response/result verdict conflict for " + files.ResultPath)

		return
	}

	s.finalClass[cell] = class
	s.recordFinalClass(cell, files, class)
}

// recordFinalClass converts a verdict class into observations.
func (s *partitionState) recordFinalClass(cell cellID, files *cellFiles, class VerdictClass) {
	switch class {
	case ClassPassCanonical:
		s.observe(cell, ObsPass, files.ResponsePath, "")
	case ClassFailCanonical:
		s.failedCells[cell] = true
		s.observe(cell, ObsFailSemantic, files.ResponsePath, "final verdict")
	case ClassNonCanonical:
		s.observe(cell, ObsNonCanonical, files.ResponsePath, "")
	case ClassProtocolNoMarker, ClassProtocolBadYAML:
		s.observe(cell, ObsFailProtocol, files.ResponsePath, string(class))

		if files.ResultPath != "" {
			s.diagnose("stale result survives protocol failure: " + files.ResultPath)
		}
	}
}

// classifyPreFix recovers the overwritten pre-fix verdict from the
// fix-generator user prompt ("**Actual:** fail").
func (s *partitionState) classifyPreFix(cell cellID, files *cellFiles) {
	if files.IterUserPath == "" {
		if files.HasPromptsMD && s.finalClass[cell] != ClassFailCanonical {
			s.observe(cell, ObsFailUnclassed, s.partition, "fix generated but pre-fix verdict unrecoverable")
		}

		return
	}

	actual, found := actualLineValue(files.IterUserPath)
	if !found {
		s.observe(cell, ObsFailUnclassed, files.IterUserPath, "no **Actual:** line")

		return
	}

	switch strings.ToLower(actual) {
	case "fail":
		s.failedCells[cell] = true
		s.observe(cell, ObsFailSemantic, files.IterUserPath, "pre-fix verdict")
	case "":
		s.observe(cell, ObsFailProtocol, files.IterUserPath, "pre-fix protocol-coerced fail")
	default:
		s.observe(cell, ObsNonCanonical, files.IterUserPath, "pre-fix actual: "+actual)
	}
}

// resultConflicts reports whether the surviving result.yaml carries a
// canonical verdict that contradicts the re-parsed response: the
// evaluator writes response before result, so a partial write leaves this.
func (s *partitionState) resultConflicts(files *cellFiles, responseClass VerdictClass) bool {
	if files.ResultPath == "" || !responseClass.Credits() {
		return false
	}

	data, err := disk.Read(files.ResultPath)
	if err != nil {
		return false
	}

	var result evaluatorResult

	err = yaml.Unmarshal([]byte(stripMarkdownFences(string(data))), &result)
	if err != nil {
		return false // malformed result: response remains the verdict source
	}

	resultClass := classifyAnswerNode(result.Answer)
	if !resultClass.Credits() {
		return false
	}

	return resultClass != responseClass
}

// warnAfterLastSave reports whether the log shows a protocol WARN for
// this cell's result path after that path's last successful save — the
// signature of a stale surviving response/result pair.
func (s *partitionState) warnAfterLastSave(cell cellID, sanitizedPath string) bool {
	if s.segment == nil {
		return false
	}

	wanted := "/" + resultFileName(cell, sanitizedPath)
	lastSave, lastWarn := -1, -1

	for eventIdx, event := range s.segment.Events {
		if !strings.HasSuffix(event.File, wanted) {
			continue
		}

		switch event.Kind {
		case evResultSaved:
			lastSave = eventIdx
		case evWarnProtocol:
			lastWarn = eventIdx
		case evLoadedPrompts, evFixGenerated, evFixApplied:
		}
	}

	return lastWarn > lastSave
}

var actualLineRe = regexp.MustCompile(`(?m)^\*\*Actual:\*\*[ \t]*(.*)$`)

// actualLineValue extracts the anchored "**Actual:**" field value.
func actualLineValue(path string) (string, bool) {
	data, err := disk.Read(path)
	if err != nil {
		return "", false
	}

	m := actualLineRe.FindStringSubmatch(string(data))
	if m == nil {
		return "", false
	}

	return strings.TrimSpace(m[1]), true
}

// resultFileName reconstructs the evaluator result filename for a cell.
func resultFileName(cell cellID, sanitizedPath string) string {
	return fmt.Sprintf("%02d-%s-checklist-%s-result.yaml", cell.Index, cell.Subject, sanitizedPath)
}
