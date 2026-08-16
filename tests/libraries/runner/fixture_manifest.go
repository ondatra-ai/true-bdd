package runner

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrInputRequired is returned when fixture.yaml has no `input` field
// — the runner needs to know which directory to overlay onto its tmpdir.
var ErrInputRequired = errors.New("fixture.yaml: input is required")

// ErrTimeoutInvalid is returned when fixture.yaml declares a `timeout`
// that time.ParseDuration rejects, or one that is not positive.
var ErrTimeoutInvalid = errors.New("fixture.yaml: timeout is not a positive duration")

// ErrJudgeSpecRequired is returned when fixture.yaml has no
// `expected.judge` field. Every fixture must declare a judge rubric —
// without one the Claude verdict step has nothing to compare against.
var ErrJudgeSpecRequired = errors.New("fixture.yaml: expected.judge is required")

// FixtureManifest is the on-disk shape of fixture.yaml: the DATA one
// scenario runs against, plus the rubric a new model response is graded
// by.
//
// What the run DOES — the invocation, the exit code it must return, the
// stdout it must print — is not here. That is behaviour, it lives in the
// scenario registry, and tests/libraries/bddgo binds it to this tree
// through the fixture name in the scenario's Given step. One fact, one
// document: a manifest that also declared the command would be a second
// place to change it, and the two would disagree the first time somebody
// changed only one.
type FixtureManifest struct {
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

	// ChecklistPrompts, when present, makes the runner GENERATE a
	// filtered checklist inside the tmpdir during prep: for each
	// `<stem>: [snippets...]` entry, the overlaid
	// `true-bdd/checklists/<stem>.yaml` is rewritten to contain only
	// the prompts whose Q text contains one of the snippets
	// (whitespace-collapsed substring match). This lets a fixture walk
	// a single shipped prompt WITHOUT checking in a copy of it — the
	// filtered file is derived from the live shipped checklist, so a
	// shipped edit flows into the fixture automatically.
	//
	// Rules (enforced at load/prep time): each stem must equal the
	// hyphenated checklist stem of `cmd`; each snippet must match
	// exactly one prompt; two snippets must not resolve to the same
	// prompt; the fixture must not ALSO ship an input override for the
	// same checklist file. Selection keeps source-document order.
	ChecklistPrompts map[string][]string `yaml:"checklist_prompts,omitempty"`

	// Timeout caps the CLI run for this fixture alone (prep and
	// teardown have their own budgets). Blank or absent means the
	// suite default, which is deliberately tight: past a few minutes a
	// checklist fixture is not slow, it is wrong. Raise it only for a
	// fixture whose CLI invocation legitimately does heavy external
	// work — building a Docker image, re-running a browser suite.
	//
	// Any non-blank value must be a POSITIVE duration that
	// time.ParseDuration accepts, e.g. "15m"; zero, negative and
	// malformed values are rejected at load time with
	// ErrTimeoutInvalid rather than silently falling back to the
	// default. Optional.
	Timeout string `yaml:"timeout,omitempty"`

	// Expected is the bundle of assertion strategies applied after
	// the CLI exits.
	Expected Expected `yaml:"expected"`
}

// Expected is what grades a run whose output is NEW — live and record
// mode, where a model just wrote something no recording covers.
//
// Exit code and stdout are absent on purpose: those are scenario steps
// now, asserted by the suite's step definitions on every run including
// replay. What is left is the one judgement no comparison can make.
type Expected struct {
	// Judge is the markdown rubric handed to the Claude judge. Required.
	Judge string `yaml:"judge"`
}

// LoadFixtureManifest reads and validates a fixture.yaml.
func LoadFixtureManifest(path string) (*FixtureManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture.yaml: %w", err)
	}

	var manifest FixtureManifest

	err = yaml.Unmarshal(data, &manifest)
	if err != nil {
		return nil, fmt.Errorf("parse fixture.yaml: %w", err)
	}

	if strings.TrimSpace(manifest.Input) == "" {
		return nil, ErrInputRequired
	}

	if strings.TrimSpace(manifest.Expected.Judge) == "" {
		return nil, ErrJudgeSpecRequired
	}

	err = rejectEmptyFilterDeclaration(data, &manifest)
	if err != nil {
		return nil, err
	}

	_, err = parseFixtureTimeout(manifest.Timeout)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}

// rejectEmptyFilterDeclaration fails a manifest whose checklist_prompts
// KEY is present but null/empty: a declared filter that selects nothing
// would silently run the full checklist.
func rejectEmptyFilterDeclaration(data []byte, manifest *FixtureManifest) error {
	if len(manifest.ChecklistPrompts) > 0 {
		return nil
	}

	var doc yaml.Node

	err := yaml.Unmarshal(data, &doc)
	if err != nil {
		return fmt.Errorf("re-parsing fixture.yaml: %w", err)
	}

	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}

	root := doc.Content[0]
	for idx := 0; idx+1 < len(root.Content); idx += 2 {
		if root.Content[idx].Value == "checklist_prompts" {
			return ErrFilterDeclaredEmpty
		}
	}

	return nil
}

// parseFixtureTimeout turns the optional `timeout:` string into a
// duration. An empty value means "use the suite default" and yields 0,
// which callers read as "unset".
func parseFixtureTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}

	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", ErrTimeoutInvalid, trimmed, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("%w: %q is not positive", ErrTimeoutInvalid, trimmed)
	}

	return parsed, nil
}

// CompileStdoutRegexes turns a scenario's stdout patterns into matchers,
// failing on the first one Go's regexp package rejects.
func CompileStdoutRegexes(patterns []string) ([]*regexp.Regexp, error) {
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
