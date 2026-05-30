package architecture

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/infrastructure/config"
)

// ErrNoServices signals that the architecture YAML has no
// `architecture.services:` list to walk.
var ErrNoServices = errors.New(
	"architecture file has no services to walk",
)

// TestConfig is one `quality_gate.tests.<layer>` block from
// architecture.yaml. Mirrors the testrunner.Config shape; converted at
// the LoadItems boundary.
type TestConfig struct {
	Path       string `yaml:"path"`
	Framework  string `yaml:"framework"`
	ConfigFile string `yaml:"config,omitempty"`
	Pattern    string `yaml:"pattern,omitempty"`
}

// ServiceTests bundles the three test-layer configs declared under
// one service's `quality_gate.tests:`.
type ServiceTests struct {
	E2E         TestConfig `yaml:"e2e"`
	Integration TestConfig `yaml:"integration"`
	Unit        TestConfig `yaml:"unit"`
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

// Loader reads architecture.yaml into a sorted Architecture struct.
type Loader struct {
	config *config.ViperConfig
}

// NewLoader builds a Loader. The config is held so callers can fall
// back to the configured default path when no override is passed.
func NewLoader(cfg *config.ViperConfig) *Loader {
	return &Loader{config: cfg}
}

// Load reads the YAML architecture file at `path`, decodes only the
// service.quality_gate.tests blocks the build-code pipeline needs, and
// returns one Service per architecture entry sorted by name.
func (l *Loader) Load(path string) (*Architecture, error) {
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

	slog.Info("Loaded architecture",
		"file", path,
		"services", len(services),
	)

	return &Architecture{Services: services}, nil
}
