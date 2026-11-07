package commands

import (
	"context"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
	"log/slog"
)

func (e *buildStepsExecution) checkConvergence(_ context.Context, _ []*BuildStepsItem) (bool, error) {
	scenarios, err := e.deps.RegistryLoader.Load(e.registryFile)
	if err != nil {
		return false, fmt.Errorf("reload Registry: %w", err)
	}

	answer, err := stepcoverage.Scan(scenarios, ".")
	if err != nil {
		return false, err
	}

	if findings := stepCoverageFindings(answer); len(findings) > 0 {
		return false, fmt.Errorf("%w: %v", ErrMissingStepDefinitions, findings)
	}

	slog.Info("Step coverage passed", "examined", len(answer.Examined))

	return true, nil
}
