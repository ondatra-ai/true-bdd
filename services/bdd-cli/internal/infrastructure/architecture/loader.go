package architecture

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// ErrNoServices signals that the architecture YAML has no
// `architecture.services:` list — nothing for a fix to write into.
var ErrNoServices = errors.New(
	"architecture file has no services to walk",
)

// ErrNoTestSuites signals no `architecture.testing.suites:` declared. A
// spec that never says how tests run is a startup refusal, not a walk
// over zero items quietly reported as green.
var ErrNoTestSuites = errors.New(
	"architecture file has no test suites to walk",
)

// ErrUnknownSuiteService signals a suite whose `service:` names no entry
// in `services[]`: that name is what grants the fix applier its write
// root, so an uncaught typo would grant nothing and look like a fix that never lands.
var ErrUnknownSuiteService = errors.New(
	"test suite names a service the architecture does not declare",
)

// ErrMissingSuiteCommand signals a suite left one of the three mandatory
// `commands:` entries empty. The engine has no built-in invocation to
// fall back on, so an incomplete block is a startup error, never a silently substituted default.
var ErrMissingSuiteCommand = errors.New("test suite command is required")

// SuiteCommands is the `commands:` block on one test suite. `build code`
// executes Replay today; Record and Live are declared and validated but
// not yet reachable from any command (see ErrMissingSuiteCommand for why all three are mandatory).
type SuiteCommands struct {
	Record string `yaml:"record"`
	Replay string `yaml:"replay"`
	Live   string `yaml:"live"`
	// Coverage reports which registry steps bind to no step definition,
	// without running a scenario. The only optional command: a suite
	// without one falls back to `build tests` asking a model the same question in prose.
	Coverage string `yaml:"coverage,omitempty"`
}

// Suite is one entry under `architecture.testing.suites[]`, mirroring
// testrunner.Config (converted at the LoadItems boundary). Service names
// the ONE service this suite exercises — a suite covering two is two suites, since a fix prompt needs one root.
type Suite struct {
	Name       string        `yaml:"name"`
	Service    string        `yaml:"service"`
	Path       string        `yaml:"path"`
	Framework  string        `yaml:"framework"`
	ConfigFile string        `yaml:"config,omitempty"`
	Pattern    string        `yaml:"pattern,omitempty"`
	Commands   SuiteCommands `yaml:"commands"`
}

// Label is how a suite is named in progress lines and refusals:
// `<service>/<suite>`. Both halves: a repo with two suites over one
// service, or one suite over two, is unambiguous only when the pair prints.
func (s Suite) Label() string {
	return s.Service + "/" + s.Name
}

// Service is one entry in `architecture.services[]`. Only the three
// fields the build pipeline reads are decoded — `path` above all, since
// it is the one root a fix applier is granted.
type Service struct {
	Name     string `yaml:"name"`
	Path     string `yaml:"path"`
	Language string `yaml:"language"`
}

// Architecture is the loaded view of architecture.yaml — only the
// fields the build pipeline needs are decoded.
type Architecture struct {
	Suites   []Suite
	Services []Service
}

// ServicePath returns the declared source root of the named service.
// Used to point a suite's failures at the one tree a fix may edit.
func (a *Architecture) ServicePath(name string) (string, bool) {
	for _, svc := range a.Services {
		if svc.Name == name {
			return svc.Path, true
		}
	}

	return "", false
}

// rawTesting mirrors the `architecture.testing:` block — the single
// place a repository states how its tests run.
type rawTesting struct {
	Suites []Suite `yaml:"suites"`
}

// rawDef mirrors the top-level `architecture:` block. Every other key a
// host writes under it — dependencies, patterns, design systems — is
// spec for its own readers and stays undecoded.
type rawDef struct {
	Testing  rawTesting `yaml:"testing"`
	Services []Service  `yaml:"services"`
}

// rawArchitecture mirrors the top-level shape of architecture.yaml.
type rawArchitecture struct {
	Architecture rawDef `yaml:"architecture"`
}

// Load reads the YAML architecture file at `path`, decodes the testing
// suites and service source roots the build pipeline needs, and returns
// both sorted by name. Purely path-driven — the cmd layer resolves the path before calling it.
func Load(path string) (*Architecture, error) {
	data, err := disk.Read(path)
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

	if len(raw.Architecture.Testing.Suites) == 0 {
		return nil, fmt.Errorf("%s: %w", path, ErrNoTestSuites)
	}

	arch := &Architecture{
		Suites:   append([]Suite(nil), raw.Architecture.Testing.Suites...),
		Services: append([]Service(nil), raw.Architecture.Services...),
	}

	sort.Slice(arch.Suites, func(i, j int) bool {
		return arch.Suites[i].Name < arch.Suites[j].Name
	})
	sort.Slice(arch.Services, func(i, j int) bool {
		return arch.Services[i].Name < arch.Services[j].Name
	})

	err = validateSuites(path, arch)
	if err != nil {
		return nil, err
	}

	slog.Info("Loaded architecture",
		"file", path,
		"services", len(arch.Services),
		"suites", len(arch.Suites),
	)

	return arch, nil
}

// validateSuites refuses a spec whose suites cannot be walked — unknown
// service, or an empty `commands:` entry — before the first subprocess
// spawns; messages name the full YAML path since a bare error can't tell suites apart.
func validateSuites(path string, arch *Architecture) error {
	for _, suite := range arch.Suites {
		_, known := arch.ServicePath(suite.Service)
		if !known {
			return fmt.Errorf(
				"%s: architecture.testing.suites[%s]: service %q: %w",
				path, suite.Name, suite.Service, ErrUnknownSuiteService,
			)
		}

		err := suite.Commands.validate(path, suite.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

// validate reports the first empty command in the block, named by the
// key a host has to fill in.
func (c SuiteCommands) validate(path, suite string) error {
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
			"%s: architecture.testing.suites[%s]: commands.%s: %w",
			path, suite, mode.key, ErrMissingSuiteCommand,
		)
	}

	return nil
}
