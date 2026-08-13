package inventory_test

// §1c typed-epic validity golden (plan §1/§4). Epic parseability must come
// from the REAL typed models.EpicDocument decoder — the SAME rejecting
// ScenarioStep semantics the us create loader uses — not an id-only probe.
// A YAML-valid epic whose story declaration carries a scalar (invalid) step
// is therefore INVALID, so the harness never renders Create controls where
// us create would itself reject the epic. Compile-safe: asserts the
// existing `status` field with the TARGET value, so it is red (the current
// id-only probe reports `parseable`) until the typed decoder lands.

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
	models "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models"
	"gopkg.in/yaml.v3"
)

// typedInvalidEpic is YAML-valid but its declared story has a scalar
// scenario step (`- 42`) the real story.ScenarioStep decoder rejects.
const typedInvalidEpic = `epic:
  id: 70
  name: Typed Invalid Epic
  status: PLANNED
stories:
  - id: "70.1"
    title: Rejected Story
    as_a: user
    i_want: x
    so_that: y
    acceptance_criteria:
      - id: AC-1
        description: rule
        steps:
          - 42
`

func TestGoldenTypedInvalidEpicIsInvalidWithNoRows(t *testing.T) {
	t.Parallel()

	// Sanity: the REAL epic model rejects this document (the semantics the
	// scanner must mirror).
	var doc models.EpicDocument

	err := yaml.Unmarshal([]byte(typedInvalidEpic), &doc)
	if err == nil {
		t.Fatal("real EpicDocument model unexpectedly accepted a scalar scenario step")
	}

	folder := honestyTree(t, map[string]string{
		"docs/prd/epics/epic-70-typed-invalid.yaml": typedInvalidEpic,
	})

	epic := gEpicOf(t, scanJSON(t, folder), "epic-70-typed-invalid.yaml")

	if epic.Status != inventory.EpicInvalid {
		t.Fatalf("typed-invalid epic status = %q, want invalid (typed decoder mirrors us create)", epic.Status)
	}

	// An invalid epic carries no Create-addressable story rows.
	if len(epic.Stories) != 0 {
		t.Fatalf("typed-invalid epic has %d rows, want 0", len(epic.Stories))
	}
}
