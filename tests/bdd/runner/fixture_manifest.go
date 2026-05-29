package runner

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrCmdRequired is returned when fixture.yaml has no `cmd` field — the
// runner needs an invocation to exec.
var ErrCmdRequired = errors.New("fixture.yaml: cmd is required")

// ErrInputRequired is returned when fixture.yaml has no `input` field
// — the runner needs to know which directory to overlay onto its tmpdir.
var ErrInputRequired = errors.New("fixture.yaml: input is required")

// ErrJudgeSpecRequired is returned when fixture.yaml has no
// `expected.judge` field. Every fixture must declare a judge rubric —
// without one the Claude verdict step has nothing to compare against.
var ErrJudgeSpecRequired = errors.New("fixture.yaml: expected.judge is required")

// FixtureManifest is the on-disk shape of fixture.yaml: it declares
// what to run (cmd, input, answers, prep) and what to assert (expected).
type FixtureManifest struct {
	// Cmd is the single-line CLI invocation. Required.
	Cmd string `yaml:"cmd"`

	// Input is the path (relative to the fixture's own directory) of
	// the directory tree to overlay onto the runner's tmpdir AFTER
	// the repo-layer pre-populate. Required; conventionally "input".
	Input string `yaml:"input"`

	// Answers is piped verbatim to the subprocess's stdin (one line
	// per prompt for the `--fix` interactive loop). Empty means no
	// stdin is piped.
	Answers string `yaml:"answers"`

	// Prep is a list of shell commands run in the tmpdir AFTER the
	// input overlay and BEFORE the pre-run snapshot. Used to install
	// dependencies (`npm install`, `playwright install`) so the
	// fixture's CLI invocation can shell out to external test runners.
	// Side effects of prep are excluded from the post-run diff handed
	// to the judge. Each entry is executed via `bash -c`. Optional.
	Prep []string `yaml:"prep,omitempty"`

	// Teardown is a list of shell commands run in the tmpdir AFTER the
	// post-run snapshot — so their side effects never reach the judge's
	// diff — and AFTER the CLI exits, regardless of whether the run
	// succeeded, failed, or hit the fixture timeout. Used to tear down
	// long-lived external resources the fixture started (Docker compose
	// stacks, background daemons) so the next run starts from a clean
	// slate. Each entry is executed via `bash -c` against a fresh,
	// teardown-only timeout (independent of the fixture timeout, so
	// teardown still runs when the CLI itself was killed). Failures are
	// logged to stderr but do NOT mask the primary run result. Optional.
	Teardown []string `yaml:"teardown,omitempty"`

	// Expected is the bundle of assertion strategies applied after
	// the CLI exits.
	Expected Expected `yaml:"expected"`
}

// Expected mirrors the assertion strategies declared under
// `expected:` in fixture.yaml. Each field corresponds to one strategy
// applied by checks.go / judge.go.
type Expected struct {
	// ExitCode is the exit status the CLI must return. Defaults to 0
	// when absent.
	ExitCode int `yaml:"exit_code"`

	// StdoutRegex is a list of Go regexp patterns. Each pattern is
	// asserted to match somewhere in captured stdout. Absent or empty
	// means no stdout assertions.
	StdoutRegex []string `yaml:"stdout_regex"`

	// Judge is the markdown rubric handed to the Claude judge. Required.
	Judge string `yaml:"judge"`
}

// LoadFixtureManifest reads and validates a fixture.yaml. It also
// compiles each StdoutRegex entry so a bad pattern fails at load time
// rather than at assertion time.
func LoadFixtureManifest(path string) (*FixtureManifest, []*regexp.Regexp, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read fixture.yaml: %w", err)
	}

	var manifest FixtureManifest

	err = yaml.Unmarshal(data, &manifest)
	if err != nil {
		return nil, nil, fmt.Errorf("parse fixture.yaml: %w", err)
	}

	if strings.TrimSpace(manifest.Cmd) == "" {
		return nil, nil, ErrCmdRequired
	}

	if strings.TrimSpace(manifest.Input) == "" {
		return nil, nil, ErrInputRequired
	}

	if strings.TrimSpace(manifest.Expected.Judge) == "" {
		return nil, nil, ErrJudgeSpecRequired
	}

	regexes, err := compileStdoutRegexes(manifest.Expected.StdoutRegex)
	if err != nil {
		return nil, nil, err
	}

	return &manifest, regexes, nil
}

func compileStdoutRegexes(patterns []string) ([]*regexp.Regexp, error) {
	var regexes []*regexp.Regexp

	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile regex %q: %w", pattern, err)
		}

		regexes = append(regexes, re)
	}

	return regexes, nil
}
