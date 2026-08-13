package inventory

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
)

// budgetEnvVar is the pinned env knob that drives the request-budget
// degrade ladder without a running server (plan §1a). It is the fit budget
// the remote otherwise derives from the server's negotiated inventory
// limit; exposing it as an override makes the ladder unit-testable.
const budgetEnvVar = "TRUE_BDD_INVENTORY_BUDGET_BYTES"

// UnavailableLimitTooSmall is the session-level cannot-fit state: the
// negotiated request budget is below the always-fit floor, so not even the
// document-chip floor snapshot can be carried.
const UnavailableLimitTooSmall = "limit_too_small"

// minFloorSampleChips is the number of saturated sample document chips used
// to over-estimate the worst-case bounded floor size in MinInventoryFloor.
const minFloorSampleChips = 12

// MinInventoryFloor is the smallest snapshot-fit budget for which the
// always-fit FLOOR (document chips + global counts, NO per-epic entries and
// NO unbounded error strings) is still emitted. A budget below it is a
// TERMINAL cannot-fit condition: the snapshot is marked
// unavailable("limit_too_small") rather than degraded forever (plan §1a).
//
// Unlike the previous hard-coded 200, it is COMPUTED from the worst-case
// bounded floor (all document chips at their longest status + saturated
// global counts), so it is an honest lower bound on what the floor actually
// serializes to — finding 1. The remote adds the request envelope to it to
// reject an impossible server cap out-of-band.
//
//nolint:gochecknoglobals // a computed constant threshold, not mutable state
var MinInventoryFloor = computeMinInventoryFloor()

// computeMinInventoryFloor serializes a saturated worst-case bounded floor
// (a generous over-estimate of the document-chip vocabulary + max counts),
// so MinInventoryFloor is an honest ceiling on the floor's real byte size
// regardless of the concrete document keys a scan produces.
func computeMinInventoryFloor() int {
	docs := make(map[string]string, minFloorSampleChips)
	// The sample chips have keys at least as long as the real vocabulary
	// (checklist-build-tests is the longest at 21 chars), each at the
	// longest status string.
	for index := range minFloorSampleChips {
		docs["checklist-saturated-key-"+strconv.Itoa(index)] = StatusPresentEmpty
	}

	floor := Snapshot{
		Documents:                docs,
		ArchitecturePathMismatch: true,
		Epics:                    []Epic{},
		Totals:                   Totals{Epics: math.MaxInt32, DeclaredStories: math.MaxInt32},
		SnapshotTruncated:        true,
		StoriesOmitted:           math.MaxInt32,
	}

	return serializedLen(floor)
}

// envBudget reads TRUE_BDD_INVENTORY_BUDGET_BYTES; 0 (unset/invalid) means
// unlimited (no fitting).
func envBudget() int {
	raw := os.Getenv(budgetEnvVar)
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}

	return value
}

// serializedLen is the JSON byte length of a value — the size the fit
// ladder compares against the budget.
func serializedLen(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}

	return len(data)
}

// snapshotBuilder assembles a budget-fitted snapshot INCREMENTALLY, adding
// epics and stories one at a time and retaining each only to the extent it
// fits (plan §1a / finding 2). It maintains the exact serialized size of the
// snapshot-so-far as a running counter (`used`), updated by per-item deltas,
// so it never re-marshals the whole snapshot — killing the previous O(n²)
// whole-snapshot re-marshal loop — and it never holds more than the budget
// (plus the one story currently being fitted) of retained bytes.
type snapshotBuilder struct {
	snap      Snapshot
	base      Snapshot // documents + totals + arch fields, no epics (for the floor)
	used      int
	budget    int
	full      bool // no further item fits; stop retaining
	truncated bool // any degradation happened (drives snapshot_truncated)
}

// newSnapshotBuilder seeds the builder from the base snapshot (all
// top-level fields except epics) and measures it ONCE.
func newSnapshotBuilder(base Snapshot, budget int) *snapshotBuilder {
	base.Epics = []Epic{}

	return &snapshotBuilder{
		snap:   base,
		base:   base,
		used:   serializedLen(base),
		budget: budget,
	}
}

// addEpicHeader appends an epic header (with an empty stories list) when it
// still fits. It returns false — and latches `full` — when the header does
// not fit, so no later epic is added out of size order.
func (b *snapshotBuilder) addEpicHeader(header Epic) bool {
	if b.full {
		return false
	}

	header.Stories = []Story{}

	delta := serializedLen(header)
	if len(b.snap.Epics) > 0 {
		delta++ // the array comma
	}

	if b.used+delta > b.budget {
		b.full = true
		b.truncated = true

		return false
	}

	b.snap.Epics = append(b.snap.Epics, header)
	b.used += delta

	return true
}

// addStory fits one story into the CURRENT (last-added) epic, degrading the
// story per-row largest-first: full → drop raw (raw_omitted) → drop file
// content (content_omitted). The declared-content fallback is never dropped
// (it is the "every story openable" guarantee). A row that cannot fit even
// minimally is truncated (latching `full`), counted later in
// stories_omitted.
func (b *snapshotBuilder) addStory(story Story) bool {
	if b.full || len(b.snap.Epics) == 0 {
		if len(b.snap.Epics) > 0 {
			b.truncated = true
		}

		return false
	}

	epic := &b.snap.Epics[len(b.snap.Epics)-1]

	comma := 0
	if len(epic.Stories) > 0 {
		comma = 1
	}

	for _, candidate := range degradedForms(story) {
		delta := serializedLen(candidate) + comma
		if b.used+delta <= b.budget {
			epic.Stories = append(epic.Stories, candidate)
			b.used += delta

			if candidate.RawOmitted || candidate.ContentOmitted {
				b.truncated = true
			}

			return true
		}
	}

	b.full = true
	b.truncated = true

	return false
}

// degradedForms yields a story's retained variants in largest-first order:
// the full row, the row without its raw (raw_omitted), and the minimal row
// without raw OR file content (both omitted). Each later form is strictly
// smaller, so the caller keeps the largest that fits — preserving the
// per-story invariant that content is never omitted while raw is retained.
func degradedForms(story Story) []Story {
	forms := []Story{story}

	if story.Raw != "" {
		noRaw := story
		noRaw.Raw = ""
		noRaw.RawTruncated = false
		noRaw.RawOmitted = true
		noRaw.OmissionReason = omissionReason(&noRaw)
		forms = append(forms, noRaw)
	}

	minimal := story
	minimal.Raw = ""
	minimal.RawTruncated = false

	if story.Raw != "" {
		minimal.RawOmitted = true
	}

	if story.Content != nil {
		minimal.Content = nil
		minimal.ContentOmitted = true
	}

	minimal.OmissionReason = omissionReason(&minimal)

	// Only append the minimal form when it differs from an earlier form
	// (a story with neither raw nor content has a single form).
	if minimal.RawOmitted || minimal.ContentOmitted {
		forms = append(forms, minimal)
	}

	return forms
}

// finalize records the omission counts, sets snapshot_truncated when any
// degradation happened, and collapses to the always-fit floor when NOTHING
// fit (or, defensively, when the running size somehow exceeds the budget).
// totalRenderable is the true count of declared story rows the full scan
// would render, so stories_omitted is honest even though truncated rows are
// never retained.
func (b *snapshotBuilder) finalize(totalRenderable int) Snapshot {
	retained := countDeclaredStories(b.snap.Epics)

	omitted := totalRenderable - retained
	if omitted < 0 {
		omitted = 0
	}

	b.snap.StoriesOmitted = omitted

	// Nothing fit — collapse to the bounded floor (or the cannot-fit state).
	if retained == 0 && totalRenderable > 0 {
		return floorSnapshot(b.base, b.budget, omitted)
	}

	if b.truncated || omitted > 0 {
		b.snap.SnapshotTruncated = true
	}

	// Defensive exact re-check: the incremental `used` is exact, but a final
	// verification guarantees the serialized body never exceeds the budget.
	if serializedLen(b.snap) > b.budget {
		return floorSnapshot(b.base, b.budget, omitted)
	}

	return b.snap
}

// floorSnapshot collapses to the always-fit floor: document chips + global
// counts, NO per-epic entries and NO unbounded document error strings
// (finding 1). When even that bounded floor cannot fit the budget — a cap
// below MinInventoryFloor — only the minimal honest cannot-fit signal is
// carried (document chips dropped too), reported as
// unavailable="limit_too_small".
func floorSnapshot(base Snapshot, budget int, storiesOmitted int) Snapshot {
	floor := Snapshot{
		Documents:                base.Documents,
		ArchitecturePathMismatch: base.ArchitecturePathMismatch,
		Epics:                    []Epic{},
		Totals:                   base.Totals,
		SnapshotTruncated:        true,
		StoriesOmitted:           storiesOmitted,
	}
	// DocumentErrors and the architecture path strings are DROPPED here: they
	// are unbounded "base strings" that would make the floor un-fittable.

	if budget < MinInventoryFloor || serializedLen(floor) > budget {
		floor.Unavailable = UnavailableLimitTooSmall
		floor.Documents = nil
	}

	return floor
}

// omissionReason maps a story's raw/content omission flags to the reason the
// modal notice renders.
func omissionReason(story *Story) string {
	switch {
	case story.RawOmitted && story.ContentOmitted:
		return "both_omitted"
	case story.ContentOmitted:
		return "content_omitted"
	case story.RawOmitted:
		return "raw_omitted"
	default:
		return ""
	}
}

// countDeclaredStories sums the rendered story rows across all epics — the
// true declared-story total the floor reports.
func countDeclaredStories(epics []Epic) int {
	total := 0
	for index := range epics {
		total += len(epics[index].Stories)
	}

	return total
}
