package scenariogen

import (
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

// TestIdentity names the generated function in the package its path owns,
// using the same module resolution and name renderer as BuildPlan.
func TestIdentity(repoRoot string, scenario *registry.RegistryScenario) (string, error) {
	shim, err := scenariosImportPath(repoRoot, filepath.Dir(scenario.Path))
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(shim, "/"+scenariosPkgDir) + "::" + funcName(scenario.ID), nil
}
