package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Paths relative to this package directory (the `go test` cwd).
const (
	shippedChecklistsDir = "../../../true-bdd/checklists"
	fixturesRootDir      = "../fixtures"
)

// soloOverrideFixtures maps each single-prompt override fixture to the
// shipped us-refine prompt (global 1-based index) its lone prompt must
// match. Every entry is an F-bearing prompt: the whole point of these
// fixtures is the fail->fix->pass chain.
func soloOverrideFixtures() map[string]int {
	return map[string]int{
		"us-refine-fix-descriptions":   2,
		"us-refine-fix-steps":          3,
		"us-refine-fix-desc-qualifier": 4,
		"us-refine-fix-step-qualifier": 5,
		"us-refine-fix-forbidden-verb": 6,
	}
}

// overrideFile is one fixture checklist override found on disk.
type overrideFile struct {
	Fixture string
	Stem    string
	Path    string
}

// discoverOverrides lists every fixture checklist override.
func discoverOverrides(t *testing.T) []overrideFile {
	t.Helper()

	pattern := filepath.Join(fixturesRootDir, "*", "input", "true-bdd", "checklists", "*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("globbing overrides: %v", err)
	}

	overrides := make([]overrideFile, 0, len(matches))

	for _, path := range matches {
		relative, relErr := filepath.Rel(fixturesRootDir, path)
		if relErr != nil {
			t.Fatalf("relativizing %s: %v", path, relErr)
		}

		overrides = append(overrides, overrideFile{
			Fixture: strings.Split(relative, string(filepath.Separator))[0],
			Stem:    strings.TrimSuffix(filepath.Base(path), ".yaml"),
			Path:    path,
		})
	}

	if len(overrides) == 0 {
		t.Fatal("no fixture checklist overrides found — glob pattern broken?")
	}

	return overrides
}

// TestOverridePromptsMatchShipped requires every prompt of every fixture
// checklist override to EvalKey-match a shipped prompt of the same
// checklist. A failure means someone edited the shipped checklist (or
// the override) without updating the other side: the affected fixture
// would silently exercise a non-shipped prompt and earn no coverage.
func TestOverridePromptsMatchShipped(t *testing.T) {
	t.Parallel()

	uni, err := LoadUniverse(shippedChecklistsDir)
	if err != nil {
		t.Fatalf("loading shipped universe: %v", err)
	}

	for _, override := range discoverOverrides(t) {
		prompts, loadErr := loadChecklistPrompts(override.Path, override.Stem)
		if loadErr != nil {
			t.Errorf("%s: loading override: %v", override.Fixture, loadErr)

			continue
		}

		if len(prompts) == 0 {
			t.Errorf("%s: override has no active prompts — the fixture would walk nothing", override.Fixture)
		}

		for _, prompt := range prompts {
			if uni.MatchOverride(override.Stem, prompt) == nil {
				t.Errorf("%s: override prompt %q does not match any shipped %s prompt",
					override.Fixture, prompt.Snippet, override.Stem)
			}
		}
	}
}

// TestSoloOverridesPinExpectedPrompt requires each single-prompt
// override fixture to contain exactly one prompt, matching exactly the
// shipped prompt it was built for, with an authored F.
func TestSoloOverridesPinExpectedPrompt(t *testing.T) {
	t.Parallel()

	uni, err := LoadUniverse(shippedChecklistsDir)
	if err != nil {
		t.Fatalf("loading shipped universe: %v", err)
	}

	for fixture, wantGlobal := range soloOverrideFixtures() {
		path := filepath.Join(fixturesRootDir, fixture, "input", "true-bdd", "checklists", "us-refine.yaml")

		prompts, loadErr := loadChecklistPrompts(path, "us-refine")
		if loadErr != nil {
			t.Errorf("%s: loading override: %v", fixture, loadErr)

			continue
		}

		if len(prompts) != 1 {
			t.Errorf("%s: want exactly 1 prompt, got %d", fixture, len(prompts))

			continue
		}

		assertSoloMatch(t, uni, fixture, prompts[0], wantGlobal)
	}
}

// assertSoloMatch checks one solo prompt against its expected shipped
// counterpart.
func assertSoloMatch(
	t *testing.T, uni *Universe, fixture string, prompt UniversePrompt, wantGlobal int,
) {
	t.Helper()

	matched := uni.MatchOverride("us-refine", prompt)
	if matched == nil {
		t.Errorf("%s: prompt %q matches no shipped us-refine prompt", fixture, prompt.Snippet)

		return
	}

	if matched.Global != wantGlobal {
		t.Errorf("%s: matches shipped q%d, want q%d", fixture, matched.Global, wantGlobal)
	}

	if !matched.HasF {
		t.Errorf("%s: matched shipped q%d has no F — fix-chain fixture is pointless", fixture, matched.Global)
	}
}

// configBlock captures the per-checklist engine knobs production reads
// from the overlaid document (runner.Run -> doc.Config).
type configBlock struct {
	Config map[string]any `yaml:"config"`
}

// TestOverrideConfigMatchesShipped requires every override to carry the
// same `config:` block as its shipped counterpart: production reads the
// knobs from the OVERLAID file, so a shipped config change that is not
// mirrored would silently change fixture engine behavior.
func TestOverrideConfigMatchesShipped(t *testing.T) {
	t.Parallel()

	for _, override := range discoverOverrides(t) {
		shipped := loadConfigBlock(t, filepath.Join(shippedChecklistsDir, override.Stem+".yaml"))
		overridden := loadConfigBlock(t, override.Path)

		if !reflect.DeepEqual(shipped.Config, overridden.Config) {
			t.Errorf("%s: config block diverges from shipped %s.yaml: shipped=%v override=%v",
				override.Fixture, override.Stem, shipped.Config, overridden.Config)
		}
	}
}

// loadConfigBlock reads just the config mapping of a checklist file.
func loadConfigBlock(t *testing.T, path string) configBlock {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var block configBlock

	err = yaml.Unmarshal(data, &block)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	return block
}
