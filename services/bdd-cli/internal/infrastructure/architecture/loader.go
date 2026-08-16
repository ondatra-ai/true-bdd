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
// `architecture.services:` list — nothing for a fix to write into.
var ErrNoServices = errors.New(
	"architecture file has no services to walk",
)

// ErrNoTestSuites signals that the architecture YAML declares no
// `architecture.testing.suites:`. A spec that never says how its tests
// run describes a project no build pipeline can act on, so it is a
// startup refusal rather than a walk over zero items reported as green.
var ErrNoTestSuites = errors.New(
	"architecture file has no test suites to walk",
)

// ErrUnknownSuiteService signals a suite whose `service:` names no entry
// in `services[]`. Caught here because that name is what decides which
// source root the fix applier may write: a typo would otherwise grant
// nothing and the walk would look like a fix that never lands.
var ErrUnknownSuiteService = errors.New(
	"test suite names a service the architecture does not declare",
)

// ErrMissingSuiteCommand signals that a declared suite left one of the
// three mandatory `commands:` entries empty. The engine has no built-in
// invocation to fall back on — how a suite runs is spec, not an engine
// default — so an incomplete block is a startup error, never a silent
// substitution.
var ErrMissingSuiteCommand = errors.New("test suite command is required")

// SuiteCommands is the `commands:` block on one test suite: the command
// line to run that suite in each of the three AI-dependency modes.
// `build code` executes Replay today; Record and Live are declared and
// validated but not yet reachable from any command.
//
// All three are mandatory so a spec can never half-declare how its
// tests run: a suite that names only the mode it happens to use today
// is a suite whose other modes silently do not exist.
type SuiteCommands struct {
	Record string `yaml:"record"`
	Replay string `yaml:"replay"`
	Live   string `yaml:"live"`
}

// Suite is one entry under `architecture.testing.suites[]`. Mirrors the
// testrunner.Config shape; converted at the LoadItems boundary.
//
// Service names the one service this suite exercises. A suite covering
// two services is two suites: a failure that names no single source
// root is a failure no fix prompt can be pointed at.
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
// `<service>/<suite>`. Both halves, because a repository with two
// suites over one service and one suite over two services are both
// legible only when the pair is printed.
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
// suites and the service source roots the build pipeline needs, and
// returns both sorted by name. Purely path-driven: the cmd layer
// resolves the path (flag override or documents.architecture_yaml)
// before calling it.
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

// validateSuites refuses a spec whose suites cannot be walked: one
// naming a service that does not exist, or one that left a `commands:`
// entry empty. Checked here rather than at first use so the whole
// document is judged before the first subprocess is spawned: a run that
// discovers one suite's tests and only then finds the next unrunnable
// has already cost minutes for a verdict the spec could have given
// immediately.
//
// The message names the full YAML path — suite and key — because a spec
// with several suites has several places this could be wrong and
// "commands.replay is required" alone names none of them.
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
