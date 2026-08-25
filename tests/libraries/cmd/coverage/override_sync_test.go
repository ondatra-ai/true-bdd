package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// Paths relative to this package directory (the `go test` cwd).
const (
	shippedChecklistsDir = "../../../../true-bdd/checklists"
	fixturesRootDir      = "../../../bdd-cli/fixtures"
	usRefineStem         = "us-refine"
)

// declaredSelection pins one fixture's checklist_prompts declaration to
// the exact shipped prompt it must resolve to: stem, short Q hash
// (content identity), and whether an authored F is required.
type declaredSelection struct {
	Stem     string
	QShort   string
	RequireF bool
}

// declaredSelections lists every fixture that generates its checklist
// via checklist_prompts, with the shipped prompt it must select.
func declaredSelections() map[string]declaredSelection {
	return map[string]declaredSelection{
		"us-refine-fix-descriptions":   {Stem: usRefineStem, QShort: "800eb66b", RequireF: true},
		"us-refine-fix-steps":          {Stem: usRefineStem, QShort: "fa3f00d3", RequireF: true},
		"us-refine-fix-desc-qualifier": {Stem: usRefineStem, QShort: "567e38e7", RequireF: true},
		"us-refine-fix-step-qualifier": {Stem: usRefineStem, QShort: "7cfe4b20", RequireF: true},
		"us-refine-fix-forbidden-verb": {Stem: usRefineStem, QShort: "bb57188d", RequireF: true},
		"us-apply-rewalk-converges":    {Stem: "us-apply", QShort: "6da54561", RequireF: true},
		"scen-check-id-filter":         {Stem: "scen-check", QShort: "30469152", RequireF: false},
	}
}

// overrideFile is one fixture checklist override found on disk.
type overrideFile struct {
	Fixture string
	Stem    string
	Path    string
}

// discoverOverrides lists every fixture checklist override, honoring
// each manifest's declared input directory.
func discoverOverrides(t *testing.T) []overrideFile {
	t.Helper()

	overrides := make([]overrideFile, 0)

	for _, fixture := range listFixtures(t) {
		selection := loadManifestSelection(t, fixture)

		inputDir := selection.Input
		if strings.TrimSpace(inputDir) == "" {
			inputDir = "input"
		}

		pattern := filepath.Join(fixturesRootDir, fixture, inputDir,
			"true-bdd", "checklists", "*.yaml")

		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("globbing overrides for %s: %v", fixture, err)
		}

		for _, path := range matches {
			overrides = append(overrides, overrideFile{
				Fixture: fixture,
				Stem:    strings.TrimSuffix(filepath.Base(path), ".yaml"),
				Path:    path,
			})
		}
	}

	// Zero overrides is a healthy state: fixtures that need a filtered
	// checklist declare checklist_prompts instead of shipping a copy.
	return overrides
}

// listFixtures enumerates the fixture directories.
func listFixtures(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(fixturesRootDir)
	if err != nil {
		t.Fatalf("reading fixtures dir: %v", err)
	}

	fixtures := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			fixtures = append(fixtures, entry.Name())
		}
	}

	return fixtures
}

// allowedOverrideFixtures is the explicit allowlist for checked-in
// checklist overrides. A fixture belongs here only when the override IS
// the behaviour under test; every other one declares checklist_prompts.
func allowedOverrideFixtures() map[string]bool {
	return map[string]bool{"build-tests-empty-checklist": true}
}

// zeroPromptOverrideFixtures names the fixtures whose override is
// deliberately empty: E2E-297 measures the refusal of a checklist with
// nothing to ask, so "would walk nothing" is what it pins.
func zeroPromptOverrideFixtures() map[string]bool {
	return map[string]bool{"build-tests-empty-checklist": true}
}

// TestNoUnsanctionedChecklistOverrides enforces the no-copied-checklists
// policy: any override file outside the allowlist fails.
func TestNoUnsanctionedChecklistOverrides(t *testing.T) {
	t.Parallel()

	for _, override := range discoverOverrides(t) {
		if !allowedOverrideFixtures()[override.Fixture] {
			t.Errorf("%s ships a checklist override (%s) — use checklist_prompts instead, "+
				"or add the fixture to allowedOverrideFixtures deliberately",
				override.Fixture, override.Path)
		}
	}
}

// TestDeclaredSelectionsComplete requires the pinned table and reality
// to agree in BOTH directions: every fixture declaring
// checklist_prompts is pinned, and every pinned fixture declares.
func TestDeclaredSelectionsComplete(t *testing.T) {
	t.Parallel()

	pinned := declaredSelections()

	for _, fixture := range listFixtures(t) {
		declares := len(loadManifestSelection(t, fixture).ChecklistPrompts) > 0

		_, isPinned := pinned[fixture]

		if declares && !isPinned {
			t.Errorf("%s declares checklist_prompts but is not pinned in declaredSelections — "+
				"add it so its target prompt is guarded", fixture)
		}

		if isPinned && !declares {
			t.Errorf("%s is pinned in declaredSelections but declares no checklist_prompts", fixture)
		}
	}
}

// TestOverridePromptsMatchShipped requires every prompt of every fixture
// checklist override to EvalKey-match a shipped prompt of the same
// checklist, or the fixture would silently exercise a non-shipped prompt.
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

		deliberatelyEmpty := zeroPromptOverrideFixtures()[override.Fixture]

		if len(prompts) == 0 && !deliberatelyEmpty {
			t.Errorf("%s: override has no active prompts — the fixture would walk nothing; "+
				"add it to zeroPromptOverrideFixtures if that is the behaviour under test",
				override.Fixture)
		}

		if len(prompts) > 0 && deliberatelyEmpty {
			t.Errorf("%s: listed in zeroPromptOverrideFixtures but its override has %d prompt(s)",
				override.Fixture, len(prompts))
		}

		for _, prompt := range prompts {
			if uni.MatchOverride(override.Stem, prompt) == nil {
				t.Errorf("%s: override prompt %q does not match any shipped %s prompt",
					override.Fixture, prompt.Snippet, override.Stem)
			}
		}
	}
}

// manifestSelection is a fixture's checklist selection plus where its
// input tree lives. Both were fixture.yaml fields; the selection now has
// its own file and the input directory is a convention.
type manifestSelection struct {
	Input            string
	ChecklistPrompts map[string][]string
}

// TestDeclaredSelectionsResolveToPinnedPrompts requires each
// checklist_prompts fixture to declare exactly one snippet resolving to
// the pinned shipped prompt (by content hash), with F where required.
func TestDeclaredSelectionsResolveToPinnedPrompts(t *testing.T) {
	t.Parallel()

	uni, err := LoadUniverse(shippedChecklistsDir)
	if err != nil {
		t.Fatalf("loading shipped universe: %v", err)
	}

	for fixture, want := range declaredSelections() {
		selection := loadManifestSelection(t, fixture)

		snippets := selection.ChecklistPrompts[want.Stem]
		if len(snippets) != 1 {
			t.Errorf("%s: want exactly 1 snippet for stem %s, got %d",
				fixture, want.Stem, len(snippets))

			continue
		}

		inputDir := selection.Input
		if strings.TrimSpace(inputDir) == "" {
			inputDir = "input"
		}

		override := filepath.Join(fixturesRootDir, fixture,
			inputDir, "true-bdd", "checklists", want.Stem+".yaml")

		_, statErr := os.Stat(override)
		if statErr == nil {
			t.Errorf("%s: declares checklist_prompts AND ships an override file %s",
				fixture, override)
		}

		assertSnippetResolves(t, uni, fixture, want, snippets[0])
	}
}

// assertSnippetResolves resolves one declared snippet against the
// shipped universe and compares the result to the pinned identity.
func assertSnippetResolves(
	t *testing.T, uni *Universe, fixture string, want declaredSelection, snippet string,
) {
	t.Helper()

	collapsed := collapseWhitespace(snippet)
	hits := make([]*UniversePrompt, 0)

	for idx := range uni.Prompts {
		prompt := &uni.Prompts[idx]
		if prompt.Checklist == want.Stem && strings.Contains(prompt.QCollapsed, collapsed) {
			hits = append(hits, prompt)
		}
	}

	if len(hits) != 1 {
		t.Errorf("%s: snippet %q resolves to %d shipped %s prompts, want exactly 1",
			fixture, snippet, len(hits), want.Stem)

		return
	}

	if got := shortHash(hits[0].QHash); got != want.QShort {
		t.Errorf("%s: snippet resolves to prompt @%s, pinned to @%s — "+
			"the shipped prompt changed; re-verify the fixture and update the pin",
			fixture, got, want.QShort)
	}

	if want.RequireF && !hits[0].HasF {
		t.Errorf("%s: resolved prompt @%s has no F — fix-chain fixture is pointless",
			fixture, want.QShort)
	}
}

// loadManifestSelection reads a fixture's checklist-prompts.yaml. A
// fixture that ships none selects nothing, which is the common case and
// not an error — only the handful narrowing a checklist have the file.
func loadManifestSelection(t *testing.T, fixture string) manifestSelection {
	t.Helper()

	selection := manifestSelection{Input: runner.DefaultInputDir}

	data, err := os.ReadFile(
		filepath.Join(fixturesRootDir, fixture, runner.ChecklistPromptsFile))
	if errors.Is(err, os.ErrNotExist) {
		return selection
	}

	if err != nil {
		t.Fatalf("%s: reading %s: %v", fixture, runner.ChecklistPromptsFile, err)
	}

	err = yaml.Unmarshal(data, &selection.ChecklistPrompts)
	if err != nil {
		t.Fatalf("%s: parsing %s: %v", fixture, runner.ChecklistPromptsFile, err)
	}

	return selection
}

// configBlock captures the per-checklist engine knobs production reads
// from the overlaid document (runner.Run -> doc.Config).
type configBlock struct {
	Config map[string]any `yaml:"config"`
}

// TestOverrideConfigMatchesShipped requires every override to carry the
// same `config:` block as its shipped counterpart: production reads the
// knobs from the overlaid file, so a drift would silently change behavior.
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
