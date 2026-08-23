package inventory

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
)

// budgetEnvVar is the env override for the request-budget ladder (plan
// §1a), standing in for the server's negotiated inventory limit so the
// ladder is unit-testable without a running server.
const budgetEnvVar = "TRUE_BDD_INVENTORY_BUDGET_BYTES"

// UnavailableLimitTooSmall is the session-level cannot-fit state: the
// negotiated request budget is below the always-fit floor, so not even the
// document-chip floor snapshot can be carried.
const UnavailableLimitTooSmall = "limit_too_small"

// minFloorSampleChips is the number of saturated sample document chips used
// to over-estimate the worst-case bounded floor size in MinInventoryFloor.
const minFloorSampleChips = 12

// MinInventoryFloor is the smallest budget the always-fit floor still fits; computed, not hard-coded (finding 1).
//
//nolint:gochecknoglobals // a computed constant threshold, not mutable state
var MinInventoryFloor = computeMinInventoryFloor()

// computeMinInventoryFloor serializes a saturated worst-case floor (longest
// document-chip vocabulary + max counts) so MinInventoryFloor stays an
// honest ceiling regardless of the scan's real document keys.
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

// snapshotBuilder assembles a budget-fitted snapshot INCREMENTALLY (plan
// §1a / finding 2), retaining each epic/story only while it fits, tracked
// via a running serialized-size counter (`used`) rather than re-marshaling.
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

// addStory fits one story into the CURRENT (last-added) epic, trying
// degradedForms(story) largest-first until one fits; a story that fits
// nowhere latches `full` and is counted later via stories_omitted.
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

// degradedForms yields a story's retained variants, largest first: full,
// then raw dropped, then raw+content both dropped. DeclaredContent is never
// cleared by any form — the every-story-openable guarantee (see addStory).
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

// finalize records omission counts, marks snapshot_truncated on any
// degradation, and collapses to the always-fit floor when nothing fit (or,
// defensively, when the running size still exceeds budget).
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
// counts, no per-epic entries, no unbounded error strings (finding 1). Below
// MinInventoryFloor even that cannot fit, so only unavailable="limit_too_small" survives.
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
