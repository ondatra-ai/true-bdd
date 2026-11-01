package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// ErrStepSpecification refuses an Item whose service declares no usable
// Public interface before writing skeletons.
var ErrStepSpecification = errors.New("step specification is incomplete")

func (e *buildStepsExecution) configure(ctx context.Context, arch *architecture.Architecture,
	scenarios []*registry.RegistryScenario) error {
	if arch.Testing.Framework != testrunner.FrameworkGoTest {
		return fmt.Errorf("%w: the architecture declares %q", ErrBuildStepsFramework, arch.Testing.Framework)
	}

	err := testrunner.ValidateCommand(arch.Testing.Framework, arch.Testing.Commands.Replay)
	if err != nil {
		return err
	}

	host, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate step writer host: %w", err)
	}

	err = e.configureRoots(arch, scenarios, host)
	if err != nil {
		return err
	}

	libraries, err := arch.DownloadLibraries(ctx, host)
	if err != nil {
		return err
	}

	e.deps.FixApplier.UseLibraryReadRoots(libraries)
	e.command = testrunner.Config{Framework: arch.Testing.Framework,
		ConfigFile: arch.Testing.ConfigFile, Command: arch.Testing.Commands.Replay}

	return nil
}

func (e *buildStepsExecution) configureRoots(arch *architecture.Architecture,
	scenarios []*registry.RegistryScenario, host string) error {
	suites := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		suites = append(suites, filepath.Dir(scenario.Path))
	}

	documents := []string{e.registryFile, e.architectureFile, productDocument(e.deps.DocResolver)}

	for _, service := range arch.Services {
		if service.PublicInterface != nil && service.PublicInterface.Document != "" {
			documents = append(documents, service.PublicInterface.Document)
		}
	}

	roots, err := arch.TestWritableRoots(host, suites, documents)
	if err != nil {
		return err
	}

	paths, patterns := make([]string, 0, len(roots)), make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, root.Path)
		patterns = append(patterns, strings.TrimSuffix(root.Path, "/")+"/**")
	}

	e.deps.FixApplier.UseDeclaredWriteRoots(paths, patterns)

	return nil
}

func (e *buildStepsExecution) initializeItems(items []*BuildStepsItem) error {
	const defaultAttempts = 5

	e.items = items
	e.attempts = make(map[*BuildStepsItem]int)
	e.maxAttempts = defaultAttempts

	checklist, err := e.deps.ChecklistLoader.LoadFull(buildStepsChecklistName)
	if err != nil {
		return err
	}

	if checklist.Config != nil && checklist.Config.MaxApplyAttempts > 0 {
		e.maxAttempts = checklist.Config.MaxApplyAttempts
	}

	return nil
}
