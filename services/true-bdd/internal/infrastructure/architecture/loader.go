package architecture

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"gopkg.in/yaml.v3"
)

// ErrNoServices signals that the architecture YAML has no
// `architecture.services:` list — nothing for a fix to write into.
var ErrNoServices = errors.New(
	"architecture file has no services to walk",
)

// ErrNoFramework signals no `architecture.testing.framework:` declared.
// A spec that never says how its tests run is a startup refusal, not a
// walk over zero items quietly reported as green.
var ErrNoFramework = errors.New(
	"architecture file declares no testing framework",
)

// ErrUnknownSuiteService signals a scenario whose `service:` names no
// entry in `services[]`: that name is what grants the fix applier its
// write root, so an uncaught typo grants nothing and looks like a fix that never lands.
var ErrUnknownSuiteService = errors.New(
	"scenario names a service the architecture does not declare",
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
}

// Testing is `architecture.testing:` — how this repository runs its
// tests, stated once. There is no per-suite entry: which service a test
// belongs to and which file it lives in are the scenario's.
type Testing struct {
	Libraries []Library `yaml:"libraries,omitempty"`
	Framework string    `yaml:"framework"`
	// PrebuiltEngineArg is one opt-in argument for build tests verification.
	PrebuiltEngineArg string `yaml:"prebuilt_engine_arg,omitempty"`
	// ConfigFile is the framework's config file. Its directory is where
	// every command runs — a playwright host resolves testDir from there,
	// so without it the runner starts in the wrong tree and finds nothing.
	ConfigFile string        `yaml:"config,omitempty"`
	Commands   SuiteCommands `yaml:"commands"`
}

// Label names the testing block in progress lines and refusals.
func (t Testing) Label() string { return t.Framework }

// Service declares a production folder and any additional exact writable paths.
type Service struct {
	Libraries []Library `yaml:"libraries,omitempty"`
	Name      string    `yaml:"name"`
	Path      string    `yaml:"path"`
	Language  string    `yaml:"language"`
	Writable  []string  `yaml:"writable,omitempty"`
}

// Architecture is the loaded view of architecture.yaml — only the
// fields the build pipeline needs are decoded.
type Architecture struct {
	Testing  Testing
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
	Libraries         []Library     `yaml:"libraries"`
	PrebuiltEngineArg yaml.Node     `yaml:"prebuilt_engine_arg"`
	Framework         string        `yaml:"framework"`
	ConfigFile        string        `yaml:"config"`
	Commands          SuiteCommands `yaml:"commands"`
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

	prebuilt, err := raw.Architecture.Testing.prebuiltArgument(path)
	if err != nil {
		return nil, err
	}

	arch := &Architecture{
		Testing: Testing{
			Framework:         raw.Architecture.Testing.Framework,
			Libraries:         raw.Architecture.Testing.Libraries,
			PrebuiltEngineArg: prebuilt,
			ConfigFile:        raw.Architecture.Testing.ConfigFile,
			Commands:          raw.Architecture.Testing.Commands,
		},
		Services: append([]Service(nil), raw.Architecture.Services...),
	}

	sort.Slice(arch.Services, func(i, j int) bool {
		return arch.Services[i].Name < arch.Services[j].Name
	})

	err = arch.validateLibraries()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	err = arch.Testing.validate(path)
	if err != nil {
		return nil, err
	}

	slog.Info("Loaded architecture",
		"file", path,
		"services", len(arch.Services),
		"framework", arch.Testing.Framework,
	)

	return arch, nil
}

// validate refuses a testing block that cannot be walked — no framework,
// or an empty `commands:` entry — before the first subprocess spawns.
func (t Testing) validate(path string) error {
	if strings.TrimSpace(t.Framework) == "" {
		return fmt.Errorf("%s: architecture.testing.framework: %w", path, ErrNoFramework)
	}

	return t.Commands.validate(path, t.Framework)
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
			"%s: architecture.testing[%s]: commands.%s: %w",
			path, suite, mode.key, ErrMissingSuiteCommand,
		)
	}

	return nil
}

// ErrInvalidPrebuiltEngineArg identifies an invalid opt-in verification argument.
var ErrInvalidPrebuiltEngineArg = errors.New("prebuilt_engine_arg must contain {engine_bin} exactly once")

func (t rawTesting) prebuiltArgument(path string) (string, error) {
	if t.PrebuiltEngineArg.Kind == 0 {
		return "", nil
	}

	var argument string

	err := t.PrebuiltEngineArg.Decode(&argument)
	if err != nil {
		return "", fmt.Errorf("%s: %w: %w", path, ErrInvalidPrebuiltEngineArg, err)
	}

	if strings.Count(argument, "{engine_bin}") != 1 {
		return "", fmt.Errorf("%s: %w", path, ErrInvalidPrebuiltEngineArg)
	}

	return argument, nil
}
