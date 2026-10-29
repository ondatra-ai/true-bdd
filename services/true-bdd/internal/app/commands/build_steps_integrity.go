package commands

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
)

// ErrFixTouchedGeneratedTests identifies a writer changing the rendered assertions.
var ErrFixTouchedGeneratedTests = errors.New("the fix walk modified a generated test file")

// VerifyGeneratedTests preserves the generated-file guard for a step-writing walk.
func VerifyGeneratedTests(plan *scenariogen.Plan, root string) error {
	drifted, err := scenariogen.Verify(plan, root)
	if err != nil {
		return err
	}

	if len(drifted) == 0 {
		return nil
	}

	slog.Error("A fix turn modified a generated test file", "files", len(drifted))

	return fmt.Errorf("%w:\n%s", ErrFixTouchedGeneratedTests, scenariogen.DriftReport(drifted))
}
