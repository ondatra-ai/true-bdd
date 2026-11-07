package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
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

	err = e.prepareRuntimeAPI(ctx, arch)
	if err != nil {
		return err
	}

	err = e.configureRoots(arch, scenarios, host)
	if err != nil {
		return err
	}

	libraries, err := arch.DownloadStepLibraries(ctx, host)
	if err != nil {
		return err
	}

	e.deps.FixApplier.UseLibraryReadRoots(libraries)
	e.deps.FixGenerator.UseEvidenceReadRoots(e.readRoots, libraries)

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

	for _, imported := range []string{
		"github.com/ondatra-ai/true-bdd/pkg/testkit", "github.com/ondatra-ai/true-bdd/pkg/proxy/cli",
	} {
		if path := e.runtimeAPI[imported]; path != "" {
			documents = append(documents, path)
		}
	}

	roots, err := arch.TestWritableRoots(host, suites, documents)
	if err != nil {
		return err
	}

	readPaths, _ := stepRootPaths(roots)

	modules, err := suiteModuleFiles(host, suites)
	if err != nil {
		return err
	}

	e.configureEvidenceRoots(host, readPaths, append(slices.Clone(documents), modules...))

	roots, err = arch.StepsWritableRoots(host, suites, documents)
	if err != nil {
		return err
	}

	paths, patterns := stepRootPaths(roots)
	e.deps.FixApplier.UseDeclaredWriteRoots(paths, patterns)

	return nil
}

func suiteModuleFiles(host string, suites []string) ([]string, error) {
	modules := make([]string, 0, len(suites))
	for _, suite := range suites {
		directory, _, err := scenariogen.FindModule(filepath.Join(host, suite))
		if errors.Is(err, scenariogen.ErrNoModuleForSuite) {
			continue
		}

		if err != nil {
			return nil, err
		}

		modules = append(modules, filepath.Join(directory, "go.mod"))
	}

	return modules, nil
}

func (e *buildStepsExecution) configureEvidenceRoots(host string, paths, documents []string) {
	e.readRoots = append([]string{}, paths...)

	for _, path := range documents {
		if path != "" {
			e.readRoots = append(e.readRoots, path)
		}
	}

	for index, path := range e.readRoots {
		if !filepath.IsAbs(path) {
			e.readRoots[index] = filepath.Join(host, path)
		}
	}

	unique := make([]string, 0, len(e.readRoots))
	for _, root := range e.readRoots {
		root = filepath.Clean(root)
		if !slices.Contains(unique, root) {
			unique = append(unique, root)
		}
	}

	e.readRoots = unique

	e.deps.FixApplier.UseEvidenceReadRoots(e.readRoots)
}

func stepRootPaths(roots []architecture.WritableRoot) ([]string, []string) {
	paths, patterns := make([]string, 0, len(roots)), make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, root.Path)
		patterns = append(patterns, strings.TrimSuffix(root.Path, "/")+"/**")
	}

	return paths, patterns
}
