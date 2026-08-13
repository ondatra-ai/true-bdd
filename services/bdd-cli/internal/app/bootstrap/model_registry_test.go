package bootstrap_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// repoRoot is where ViperConfig's hardcoded ./true-bdd/true-bdd.yaml
// resolves from.
const repoRoot = "../../../../.."

// The shipped seed config is what every BDD fixture is pre-populated
// with, so a drift between its `engine.models` shape and the parser
// would break every fixture at once — and only at runtime.
func TestSeedConfigProducesAValidRegistry(t *testing.T) {
	t.Chdir(repoRoot)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatalf("read seed config: %v", err)
	}

	models, err := bootstrap.NewModelRegistry(cfg)
	if err != nil {
		t.Fatalf("seed config does not yield a valid model registry: %v", err)
	}

	for _, tier := range provider.Tiers() {
		ref, resolveErr := models.Resolve(tier)
		if resolveErr != nil {
			t.Errorf("seed config does not configure the %q tier: %v", tier, resolveErr)

			continue
		}

		if !ref.CLI.IsSupported() || ref.Model == "" {
			t.Errorf("tier %q resolved to an unusable ref %v", tier, ref)
		}
	}

	// Every role needs its own default configured — a missing key is a
	// startup error, so a seed that drops one breaks every command.
	for _, role := range provider.Roles() {
		_, err = models.Resolve(models.DefaultTier(role))
		if err != nil {
			t.Errorf("%s does not resolve: %v", role.ConfigKey(), err)
		}
	}
}
