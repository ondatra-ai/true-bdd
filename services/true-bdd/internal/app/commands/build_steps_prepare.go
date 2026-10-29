package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner"
)

// ErrStepSpecification refuses missing phrase context before writing skeletons.
var ErrStepSpecification = errors.New("step specification is incomplete")

func (e *buildStepsExecution) prepare(ctx context.Context) ([]*BuildStepsItem, error) {
	items, _, err := discoverBuildSteps(e.deps, e.registryFile, e.architectureFile, false)
	if err != nil {
		return nil, err
	}

	arch, err := architecture.Load(e.architectureFile)
	if err != nil {
		return nil, err
	}

	scenarios, err := e.deps.RegistryLoader.Load(e.registryFile)
	if err != nil {
		return nil, fmt.Errorf("load Registry: %w", err)
	}

	compiled, err := e.describe(items, arch, scenarios)
	if err != nil {
		return nil, err
	}

	err = e.configure(ctx, arch, scenarios)
	if err != nil {
		return nil, err
	}

	err = e.prepareGeneratedTests(arch, scenarios)
	if err != nil {
		return nil, err
	}

	err = writeStepSkeletons(items, compiled)
	if err != nil {
		return nil, err
	}

	slog.Info("All step skeletons written", "items", len(items))

	err = e.compiler.CompileDefinitions(e.command)
	if err != nil {
		return nil, err
	}

	err = e.initializeItems(items)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (e *buildStepsExecution) describe(items []*BuildStepsItem, arch *architecture.Architecture,
	scenarios []*registry.RegistryScenario) (*catalogue.Catalogue, error) {
	file := productDocument(e.deps.DocResolver)

	source, err := product.LoadCatalogue(file)
	if err != nil {
		return nil, err
	}

	compiled, definitions, mappings, err := compileStepSpecification(source, file, e.architectureFile)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		err := describeStepItem(item, arch, definitions, mappings, scenarios, compiled)
		if err != nil {
			return nil, err
		}
	}

	return compiled, nil
}

func writeStepSkeletons(items []*BuildStepsItem, compiled *catalogue.Catalogue) error {
	for _, item := range items {
		pattern := compiled.CapturingPattern(item.Phrase.Template)

		definition, err := stepskeleton.Write(item.Suite, item.Phrase.Template, pattern)
		if err != nil {
			return err
		}

		item.Definition = definition
		item.Findings = []string{"step not written yet: " + item.Phrase.Template}
	}

	nameStepItems(items)

	return nil
}

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

func nameStepItems(items []*BuildStepsItem) {
	counts := make(map[string]int)
	for _, item := range items {
		counts[item.Definition.Function]++
	}

	for _, item := range items {
		item.subjectID = item.Definition.Function
		if counts[item.subjectID] > 1 {
			item.subjectID = item.Suite + "-" + item.subjectID
		}
	}
}

func (e *buildStepsExecution) initializeItems(items []*BuildStepsItem) error {
	const defaultAttempts = 5

	e.items = items
	e.attempts = make(map[*BuildStepsItem]int)
	e.maxAttempts = defaultAttempts

	checklist, err := e.deps.ChecklistLoader.LoadFull("build-steps")
	if err != nil {
		return err
	}

	if checklist.Config != nil && checklist.Config.MaxApplyAttempts > 0 {
		e.maxAttempts = checklist.Config.MaxApplyAttempts
	}

	return nil
}
