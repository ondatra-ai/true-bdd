package architecture

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNoServices signals that the architecture YAML has no
// `architecture.services:` list to walk.
var ErrNoServices = errors.New(
	"architecture file has no services to walk",
)

// ErrMissingLayerCommand signals that a declared test layer left one of
// the three mandatory `commands:` entries empty. The engine has no
// built-in invocation to fall back on — how a suite runs is spec, not
// an engine default — so an incomplete block is a startup error, never
// a silent substitution.
var ErrMissingLayerCommand = errors.New("test layer command is required")

// LayerCommands is the `commands:` block on one test layer: the command
// line to run that suite in each of the three AI-dependency modes.
// `build code` executes Replay today; Record and Live are declared and
// validated but not yet reachable from any command.
//
// All three are mandatory so a spec can never half-declare how its
// tests run: a layer that names only the mode it happens to use today
// is a layer whose other modes silently do not exist.
type LayerCommands struct {
	Record string `yaml:"record"`
	Replay string `yaml:"replay"`
	Live   string `yaml:"live"`
}

// TestConfig is one `quality_gate.tests.<layer>` block from
// architecture.yaml. Mirrors the testrunner.Config shape; converted at
// the LoadItems boundary.
type TestConfig struct {
	Path       string        `yaml:"path"`
	Framework  string        `yaml:"framework"`
	ConfigFile string        `yaml:"config,omitempty"`
	Pattern    string        `yaml:"pattern,omitempty"`
	Commands   LayerCommands `yaml:"commands"`
}

// ServiceTests bundles the two test-layer configs declared under
// one service's `quality_gate.tests:`. Unit tests are implementation
// detail `build code` generates, never spec, so they have no layer here.
type ServiceTests struct {
	E2E         TestConfig `yaml:"e2e"`
	Integration TestConfig `yaml:"integration"`
}

// Layer names, mirroring the YAML keys under `quality_gate.tests:`.
// Deliberately duplicated from testrunner's constants of the same name
// rather than imported: the dependency runs the other way (testrunner
// knows nothing about this loader) and these are YAML keys, which the
// document defines and the runner only carries.
const (
	LayerIntegration = "integration"
	LayerE2E         = "e2e"
)

// Layer pairs a test layer's YAML key with its config. Returned by
// Layers so every caller — the loader's own validation and the
// build-code walk alike — enumerates layers from one place.
type Layer struct {
	Name   string
	Config TestConfig
}

// Layers returns both test layers in walk order. A layer with no
// `framework:` is undeclared: callers skip it, and the loader's command
// validation does not apply to it.
func (t ServiceTests) Layers() []Layer {
	return []Layer{
		{Name: LayerIntegration, Config: t.Integration},
		{Name: LayerE2E, Config: t.E2E},
	}
}

// Service is one entry in `architecture.services[]`.
type Service struct {
	Name     string
	Path     string
	Language string
	Tests    ServiceTests
}

// Architecture is the loaded view of architecture.yaml — only the
// fields the build-code pipeline needs are decoded.
type Architecture struct {
	Services []Service
}

// rawQualityGate mirrors the `quality_gate:` block under one service
// in architecture.yaml. Only `tests:` is decoded.
type rawQualityGate struct {
	Tests ServiceTests `yaml:"tests"`
}

// rawService mirrors one entry under `architecture.services[]`.
type rawService struct {
	Name        string         `yaml:"name"`
	Path        string         `yaml:"path"`
	Language    string         `yaml:"language"`
	QualityGate rawQualityGate `yaml:"quality_gate"`
}

// rawDef mirrors the top-level `architecture:` block.
type rawDef struct {
	Services []rawService `yaml:"services"`
}

// rawArchitecture mirrors the top-level shape of architecture.yaml.
type rawArchitecture struct {
	Architecture rawDef `yaml:"architecture"`
}

// Load reads the YAML architecture file at `path`, decodes only the
// service.quality_gate.tests blocks the build-code pipeline needs, and
// returns one Service per architecture entry sorted by name. Purely
// path-driven: the cmd layer resolves the path (flag override or
// documents.architecture_yaml) before calling it.
func Load(path string) (*Architecture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read architecture file %s: %w", path, err)
	}

	var raw rawArchitecture

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse architecture file %s: %w", path, err)
	}

	if len(raw.Architecture.Services) == 0 {
		return nil, fmt.Errorf("%s: %w", path, ErrNoServices)
	}

	services := make([]Service, 0, len(raw.Architecture.Services))

	for _, entry := range raw.Architecture.Services {
		services = append(services, Service{
			Name:     entry.Name,
			Path:     entry.Path,
			Language: entry.Language,
			Tests:    entry.QualityGate.Tests,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	err = validateLayerCommands(path, services)
	if err != nil {
		return nil, err
	}

	slog.Info("Loaded architecture",
		"file", path,
		"services", len(services),
	)

	return &Architecture{Services: services}, nil
}

// validateLayerCommands refuses a spec in which any declared test layer
// left one of the three `commands:` entries empty. Checked here rather
// than at first use so the whole document is judged before the first
// subprocess is spawned: a run that discovers one layer's tests and
// only then finds the next layer unrunnable has already cost minutes
// for a verdict the spec could have given immediately.
//
// The message names the full YAML path — service, layer, key — because
// a spec with two services and two layers has four places this could be
// wrong and "commands.replay is required" alone names none of them.
func validateLayerCommands(path string, services []Service) error {
	for _, svc := range services {
		for _, layer := range svc.Tests.Layers() {
			if layer.Config.Framework == "" {
				continue
			}

			err := layer.Config.Commands.validate(path, svc.Name, layer.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// validate reports the first empty command in the block, named by the
// key a host has to fill in.
func (c LayerCommands) validate(path, service, layer string) error {
	modes := []struct {
		key     string
		command string
	}{
		{"record", c.Record},
		{"replay", c.Replay},
		{"live", c.Live},
	}

	for _, mode := range modes {
		if strings.TrimSpace(mode.command) != "" {
			continue
		}

		return fmt.Errorf(
			"%s: services[%s].quality_gate.tests.%s: commands.%s: %w",
			path, service, layer, mode.key, ErrMissingLayerCommand,
		)
	}

	return nil
}
