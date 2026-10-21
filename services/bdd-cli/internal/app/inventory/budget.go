package inventory

import (
	"encoding/json"
	"os"
	"strconv"
)

// budgetEnvVar is the env override for the request budget (plan §1a),
// standing in for the server's negotiated inventory limit so the floor is
// unit-testable without a running server.
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
// document-chip vocabulary) so MinInventoryFloor stays an honest ceiling
// regardless of the scan's real document keys.
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
		SnapshotTruncated:        true,
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
// compares against the budget.
func serializedLen(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}

	return len(data)
}

// floorSnapshot collapses to the always-fit floor: document chips only, no
// unbounded error strings (finding 1). Below MinInventoryFloor even that
// cannot fit, so only unavailable="limit_too_small" survives.
func floorSnapshot(full Snapshot, budget int) Snapshot {
	floor := Snapshot{
		Documents:                full.Documents,
		ArchitecturePathMismatch: full.ArchitecturePathMismatch,
		SnapshotTruncated:        true,
	}
	// DocumentErrors and the architecture path strings are DROPPED here: they
	// are unbounded "base strings" that would make the floor un-fittable.

	if budget < MinInventoryFloor || serializedLen(floor) > budget {
		floor.Unavailable = UnavailableLimitTooSmall
		floor.Documents = nil
	}

	return floor
}
