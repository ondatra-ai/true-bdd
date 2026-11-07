package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// ErrRedCheck distinguishes infrastructure failures from executed step failures.
var ErrRedCheck = errors.New("red without target check failed")

// ErrScenarioGreen stops code generation when a definition does the product's work.
var ErrScenarioGreen = errors.New("scenario green without target")

func checkCodeRed(deps BuildCodeDeps, arch *architecture.Architecture, checked bool) error {
	if checked || arch.Testing.Framework != testrunner.FrameworkGoTest {
		return nil
	}

	all, required, err := codeRedIdentities(arch, deps.DocResolver)
	if err != nil || len(required) == 0 {
		return err
	}

	compiler := testrunner.NewGoTestRunner(testrunner.NewArtifacts(deps.RunDir.GetTmpOutPath()))
	command := testrunner.Config{Framework: arch.Testing.Framework,
		ConfigFile: arch.Testing.ConfigFile, Command: replayCommand(arch.Testing)}

	err = compiler.ListScenarios(command, all)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRedCheck, err)
	}

	slog.Info("Scenario list passed", "scenarios", len(all))

	identities := make([]string, 0, len(required))
	for _, identity := range all {
		if required[identity] != "" {
			identities = append(identities, identity)
		}
	}

	verdict, err := compiler.RedScenarios(command, identities)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRedCheck, err)
	}

	return reportCodeRed(verdict, required)
}

func codeRedIdentities(arch *architecture.Architecture, resolver *docs.Resolver) (
	[]string, map[string]string, error,
) {
	absent, err := servicesWithoutCode(arch)
	if err != nil || len(absent) == 0 {
		return nil, nil, err
	}

	scenarios, err := redRegistry(resolver)
	if err != nil {
		applicable, suiteErr := hasCodeRedSuite(arch)
		if suiteErr != nil {
			return nil, nil, suiteErr
		}

		if applicable {
			return nil, nil, fmt.Errorf("%w: %w", ErrRedCheck, err)
		}

		return nil, nil, nil
	}

	return redScenarioIdentities(absent, scenarios)
}

func redRegistry(resolver *docs.Resolver) ([]*registry.RegistryScenario, error) {
	file, err := resolver.Resolve(docs.KeyScenariosYAML)
	if err != nil {
		return nil, fmt.Errorf("read Registry: %w", err)
	}

	scenarios, err := registry.NewRegistryLoader().Load(file)
	if err != nil {
		return nil, fmt.Errorf("read Registry %s: %w", file, err)
	}

	return scenarios, nil
}

func servicesWithoutCode(arch *architecture.Architecture) (map[string]bool, error) {
	absent := make(map[string]bool)

	for _, service := range arch.Services {
		exists, err := service.HasCode(".")
		if err != nil {
			return nil, err
		}

		if !exists {
			absent[service.Name] = true
		}
	}

	return absent, nil
}

func redScenarioIdentities(absent map[string]bool,
	scenarios []*registry.RegistryScenario,
) ([]string, map[string]string, error) {
	all := make([]string, 0, len(scenarios))
	required := make(map[string]string)

	for _, scenario := range scenarios {
		// Only the Go suites build steps writes have the executed-definition contract.
		info, err := os.Stat(filepath.Join(filepath.Dir(scenario.Path), "steps"))
		if err != nil || !info.IsDir() {
			continue
		}

		identity, err := scenariogen.TestIdentity(".", scenario)
		if err != nil {
			return nil, nil, err
		}

		all = append(all, identity)
		if absent[scenario.Service] {
			required[identity] = scenario.ID
		}
	}

	return all, required, nil
}

func reportCodeRed(verdict testrunner.RedRun, required map[string]string) error {
	for _, identity := range verdict.Red {
		slog.Info("Scenario red without target", "scenario", required[identity], "test", identity)
	}

	if len(verdict.Green) > 0 {
		scenarios := make([]string, 0, len(verdict.Green))
		for _, identity := range verdict.Green {
			scenarios = append(scenarios, required[identity]+" ("+identity+")")
		}

		slog.Error("Scenario green without target", "scenarios", scenarios)

		return fmt.Errorf("%w: %s", ErrScenarioGreen, strings.Join(scenarios, ", "))
	}

	if len(verdict.Invalid) > 0 {
		for index, finding := range verdict.Invalid {
			identity, _, _ := strings.Cut(finding, ": not red")
			if scenarioID := required[identity]; scenarioID != "" {
				verdict.Invalid[index] = scenarioID + " (" + finding + ")"
			}
		}

		slog.Error("Red without target check failed", "findings", verdict.Invalid)

		return fmt.Errorf("%w: %s", ErrRedCheck, strings.Join(verdict.Invalid, "\n"))
	}

	slog.Info("Red without target passed", "scenarios", len(required))

	return nil
}
