package inventory_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

func TestScanSpreadDocuments(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p3-inventory-spread")

	for _, key := range []string{"config", "product", "architecture", chipRegistry, chipStoriesDir, "checklist-us-apply"} {
		wantDoc(t, snap, key, inventory.StatusPresent)
	}
}
