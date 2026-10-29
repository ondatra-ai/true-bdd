package commands

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func (e *buildStepsExecution) prepareGeneratedTests(arch *architecture.Architecture,
	scenarios []*registry.RegistryScenario) error {
	plan, err := scenariogen.BuildPlan(scenarios, arch, ".", e.registryFile)
	if err != nil {
		return err
	}

	drifted, err := scenariogen.Verify(plan, ".")
	if err != nil {
		return err
	}

	if len(drifted) > 0 {
		return fmt.Errorf("%w: %s; run true-bdd build tests --fix", ErrGeneratedTestsStale,
			scenariogen.DriftReport(drifted))
	}

	e.plan = plan

	return nil
}
