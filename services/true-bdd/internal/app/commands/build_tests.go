package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

// ErrGeneratedTestsStale identifies generated files needing regeneration.
var ErrGeneratedTestsStale = errors.New("generated test files are not what the registry renders")

// BuildTestsDeps contains the deterministic renderer's dependencies.
type BuildTestsDeps struct {
	RegistryLoader *registry.RegistryLoader
	DocResolver    *docs.Resolver
}

// RunBuildTests renders or verifies the Registry's test files without running tests or AI.
func RunBuildTests(_ context.Context, deps BuildTestsDeps, registryFile, architectureFile string, fix bool) error {
	slog.Info("BUILD TESTS")

	prep := &buildTestsPrep{registryFile: registryFile, architectureFile: architectureFile,
		productFile: productDocument(deps.DocResolver), fix: fix}

	err := prep.run(deps.RegistryLoader)
	if err != nil {
		return runner.RefuseStartup("build tests", err)
	}

	slog.Info("BUILD TESTS COMPLETE")

	return nil
}

type buildTestsPrep struct {
	architectureFile string
	productFile      string
	registryFile     string
	fix              bool
	repoRoot         string
	plan             *scenariogen.Plan
}

func (p *buildTestsPrep) run(loader *registry.RegistryLoader) error {
	scenarios, err := loader.Load(p.registryFile)
	if err != nil {
		return fmt.Errorf("failed to load requirements registry: %w", err)
	}

	arch, err := architecture.Load(p.architectureFile)
	if err != nil {
		return err
	}

	p.repoRoot, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	p.plan, err = scenariogen.BuildPlan(scenarios, arch, p.repoRoot, p.registryFile)
	if err != nil {
		return err
	}

	_, err = p.holdToCatalogue(scenarios)
	if err != nil {
		return err
	}

	return p.renderTests()
}

// renderTests writes the generated files under --fix, and refuses a run
// whose files are stale without it.
func (p *buildTestsPrep) renderTests() error {
	if p.fix {
		written, err := scenariogen.Write(p.plan, p.repoRoot)
		if err != nil {
			return err
		}

		if len(written) > 0 {
			slog.Info("Generated test files from the registry",
				"count", len(written), "files", strings.Join(written, " "))
		}

		return nil
	}

	drifted, err := scenariogen.Verify(p.plan, p.repoRoot)
	if err != nil {
		return err
	}

	if len(drifted) > 0 {
		return fmt.Errorf("%w:\n%s\n  regenerate with: true-bdd build tests --fix",
			ErrGeneratedTestsStale, scenariogen.DriftReport(drifted))
	}

	return nil
}
