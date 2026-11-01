package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

// errUndeclaredService refuses a walk's Registry in which a Scenario claiming
// a walked criterion names no service the architecture declares (#265).
var errUndeclaredService = errors.New("a merged Scenario's service: must name a service the architecture declares")

// heldDeclaredServices refuses the walk's Registry when a Scenario claiming
// one of the walked criteria leaves `service:` empty or names a service the
// architecture does not declare, whatever the merge's fix wrote.
func heldDeclaredServices(
	scenarios []*registry.RegistryScenario,
	architecturePath func() (string, error),
	items []*template.ScenarioApplyData,
) error {
	declared, names, err := declaredServiceNames(architecturePath)
	if err != nil {
		return err
	}

	walked := make(map[string]bool, len(items))
	for _, item := range items {
		walked[item.LineageScenarioID] = true
	}

	offenders := make([]string, 0)

	for _, scenario := range scenarios {
		if !claimsWalked(scenario, walked) || (scenario.Service != "" && declared[scenario.Service]) {
			continue
		}

		slog.Error("Scenario names no declared service",
			"scenario", scenario.ID, "service", scenario.Service, "declared", strings.Join(names, ","))
		offenders = append(offenders, fmt.Sprintf("%s has service %q", scenario.ID, scenario.Service))
	}

	if len(offenders) == 0 {
		return nil
	}

	return fmt.Errorf("%w (%s): %s", errUndeclaredService, strings.Join(names, ", "), strings.Join(offenders, "; "))
}

// declaredServiceNames reads the architecture's `services[].name`, in file order.
func declaredServiceNames(architecturePath func() (string, error)) (map[string]bool, []string, error) {
	path, err := architecturePath()
	if err != nil {
		return nil, nil, fmt.Errorf("find the architecture whose services a merged Scenario names: %w", err)
	}

	services, err := architecture.LoadInterfaces(path, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("read the architecture's services: %w", err)
	}

	declared := make(map[string]bool, len(services))
	names := make([]string, 0, len(services))

	for _, service := range services {
		declared[service.Name] = true
		names = append(names, service.Name)
	}

	return declared, names, nil
}

// claimsWalked reports whether the Scenario's lineage names a walked criterion.
func claimsWalked(scenario *registry.RegistryScenario, walked map[string]bool) bool {
	for _, ref := range scenario.UserStories {
		if walked[ref.ScenarioID] {
			return true
		}
	}

	return false
}
