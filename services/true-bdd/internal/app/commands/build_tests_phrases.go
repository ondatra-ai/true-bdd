package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

// ErrStepsOutsideCatalogue signals a Registry step the declared catalogue
// does not phrase, so no step definition follows from the spec. Refused at
// startup: a hand edit to the Registry cannot bypass `scen check` (#151).
var ErrStepsOutsideCatalogue = errors.New("registry steps outside the catalogue")

// holdToCatalogue holds every Registry step, claimed by a Story or not, to
// the declared catalogue as the step-phrases check does, and returns that
// catalogue for --fix. A host that declares none walks as before.
func (p *buildTestsPrep) holdToCatalogue(
	scenarios []*registry.RegistryScenario,
) (product.Catalogue, error) {
	source, err := product.LoadCatalogue(p.productFile)
	if err != nil {
		return product.Catalogue{}, fmt.Errorf("read the catalogue: %w", err)
	}

	if !source.Declared {
		slog.Info("Step phrases not checked", "reason", "the product document declares no catalogue")

		return source, nil
	}

	failures := make([]string, 0)

	for _, scenario := range scenarios {
		verdict := checks.ReviewPhrases(scenario.ID, scenario.Phrases(), source)
		if !verdict.Pass {
			failures = append(failures, verdict.Context...)
		}
	}

	if len(failures) > 0 {
		return product.Catalogue{}, fmt.Errorf("%w in %s:\n  %s",
			ErrStepsOutsideCatalogue, p.productFile, strings.Join(failures, "\n  "))
	}

	slog.Info("Step phrases checked", "scenarios", len(scenarios))

	return source, nil
}
