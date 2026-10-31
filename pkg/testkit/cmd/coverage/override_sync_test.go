package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// Paths relative to this package directory (the `go test` cwd).
const (
	shippedChecklistsDir = "../../../../true-bdd/checklists"
	fixturesRootDir      = "../../../../tests/true-bdd/fixtures"
	usRefineStem         = "us-refine"
	scenCheckStem        = "scen-check"
	usApplyStem          = "us-apply"
	// measurableQShort is the scen-check question several fixtures narrow
	// to: "whether this scenario includes at least one specific measurable criterion".
	measurableQShort = "d92aa73c"
	// vagueDescriptionQ is the us-refine vague-word-in-description prompt.
	vagueDescriptionQ = "4fa4e82c"
	// presenceQ is the us-apply "already appear as a scenario in the registry" prompt.
	presenceQ = "6da54561"
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
		"engine-claude-minimal":            {Stem: scenCheckStem, QShort: "30469152", RequireF: false},
		"engine-model-dispatch":            {Stem: scenCheckStem, QShort: "30469152", RequireF: false},
		"us-refine-fix-count":              {Stem: usRefineStem, QShort: "895981ea", RequireF: true},
		"us-refine-fix-failure-mode":       {Stem: usRefineStem, QShort: "bff23596", RequireF: true},
		"us-refine-shipped-no-failure":     {Stem: usRefineStem, QShort: "bff23596", RequireF: true},
		"us-refine-fix-implementation":     {Stem: usRefineStem, QShort: "1d63534f", RequireF: true},
		"us-refine-fix-happy-path":         {Stem: usRefineStem, QShort: "5b2f20ef", RequireF: true},
		"us-refine-fix-measurable":         {Stem: usRefineStem, QShort: "9f77fac8", RequireF: true},
		"us-refine-fix-binary":             {Stem: usRefineStem, QShort: "8ed91a7d", RequireF: true},
		"us-refine-fix-sketch":             {Stem: usRefineStem, QShort: "1fa8ce2c", RequireF: true},
		"us-refine-fix-descriptions":       {Stem: usRefineStem, QShort: "800eb66b", RequireF: true},
		"us-refine-fix-steps":              {Stem: usRefineStem, QShort: "fa3f00d3", RequireF: true},
		"us-refine-fix-desc-qualifier":     {Stem: usRefineStem, QShort: vagueDescriptionQ, RequireF: true},
		"us-refine-fix-keeps-feature":      {Stem: usRefineStem, QShort: vagueDescriptionQ, RequireF: true},
		"us-refine-fix-step-qualifier":     {Stem: usRefineStem, QShort: "882442b9", RequireF: true},
		"us-refine-fix-forbidden-verb":     {Stem: usRefineStem, QShort: "3929749d", RequireF: true},
		"us-apply-rewalk-converges":        {Stem: usApplyStem, QShort: presenceQ, RequireF: true},
		"us-apply-key-apart-from-lineage":  {Stem: usApplyStem, QShort: presenceQ, RequireF: true},
		"us-apply-names-service":           {Stem: usApplyStem, QShort: presenceQ, RequireF: true},
		"us-apply-undeclared-service":      {Stem: usApplyStem, QShort: presenceQ, RequireF: true},
		"fix-loop-exit-story":              {Stem: usRefineStem, QShort: vagueDescriptionQ, RequireF: true},
		"fix-loop-exit-registry":           {Stem: usApplyStem, QShort: presenceQ, RequireF: true},
		"scen-check-id-filter":             {Stem: scenCheckStem, QShort: "30469152", RequireF: false},
		"scen-check-fix-adds-missing-step": {Stem: scenCheckStem, QShort: "45c4a3a0", RequireF: true},
		"scen-check-fix-rewrites-description-qualifier": {
			Stem: scenCheckStem, QShort: "6fd4a11c", RequireF: true,
		},
		"scen-check-fix-rewrites-step-qualifier":     {Stem: scenCheckStem, QShort: "63aefa89", RequireF: true},
		"scen-check-fix-rewrites-forbidden-verb":     {Stem: scenCheckStem, QShort: "6d955d38", RequireF: true},
		"scen-check-fix-rewrites-implementation":     {Stem: scenCheckStem, QShort: "4ca30544", RequireF: true},
		"scen-check-fix-adds-measurable-criterion":   {Stem: scenCheckStem, QShort: measurableQShort, RequireF: true},
		"scen-check-fix-rewrites-subjective-outcome": {Stem: scenCheckStem, QShort: "17ccea8c", RequireF: true},
		"scen-check-claude-turn-fails":               {Stem: scenCheckStem, QShort: measurableQShort, RequireF: false},
		"scen-check-codex-turn-fails":                {Stem: scenCheckStem, QShort: measurableQShort, RequireF: false},
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

// overridePurpose is why a fixture's checklist override IS the behaviour
// under test, which decides what in it must still copy a shipped prompt.
type overridePurpose int

const (
	// overrideUnsanctioned is every fixture off the allowlist.
	overrideUnsanctioned overridePurpose = iota
	// overrideEmpty walks nothing: build-steps-refusals-001-empty-checklist-refused
	// and checklist-skip-003 pin the refusal of a checklist with nothing to ask.
	overrideEmpty
	// overrideRefused carries a prompt the engine refuses at startup, so
	// it walks nothing and nothing in it need be shipped.
	overrideRefused
	// overridePreserved is an unusable file init must preserve without loading.
	overridePreserved
	// overrideChecks adds `D:` prompts no shipped checklist carries yet
	// (ADR 0015); its `Q:` prompts must still copy shipped ones.
	overrideChecks
	// overrideTiers gives a shipped prompt its own `model:`, `fix_model:`
	// and `apply_model:`, which the eval key leaves out, so it still copies one.
	overrideTiers
	// overrideFixLoop shapes the fix loop under test (#168): its `F:` decides
	// what a fix asks or changes and its `config:` may bound the loop, while
	// each question or check stays a shipped one.
	overrideFixLoop
)

// allowedOverrideFixtures is the explicit allowlist for checked-in
// checklist overrides. A fixture belongs here only when the override IS
// the behaviour under test; every other one declares checklist_prompts.
func allowedOverrideFixtures() map[string]overridePurpose {
	allowed := map[string]overridePurpose{
		"init-existing":                           overridePreserved,
		"scen-check-concurrent":                   overrideChecks,
		"scen-check-reverse":                      overrideChecks,
		"start-invalid-checklist":                 overrideRefused,
		"build-steps-critique-correct":            overrideTiers,
		"build-steps-critique-operation":          overrideTiers,
		"build-steps-critique-assertion":          overrideTiers,
		"build-code-claude-replay":                overrideTiers,
		"build-code-claude-failure":               overrideTiers,
		"build-code-writer-bound":                 overrideFixLoop,
		"build-code-codex-selection":              overrideTiers,
		"build-code-codex-missing-root":           overrideTiers,
		"build-code-codex-nonzero":                overrideTiers,
		"build-code-no-confirmation-bound":        overrideFixLoop,
		"build-code-shrinking-bound":              overrideFixLoop,
		"build-code-not-applied-bound":            overrideFixLoop,
		"build-tests-empty-checklist":             overrideEmpty,
		"scen-check-every-prompt-skipped":         overrideEmpty,
		"scen-check-prompt-with-both-kinds":       overrideRefused,
		"scen-check-prompt-with-no-kind":          overrideRefused,
		"scen-check-unknown-check":                overrideRefused,
		"scen-check-model-on-check":               overrideRefused,
		"scen-check-unknown-check-parameter":      overrideRefused,
		"scen-check-fix-check-without-f":          overrideRefused,
		"scen-check-phrase-docs-unlisted":         overrideRefused,
		"scen-check-prompt-tier-unconfigured":     overrideRefused,
		"scen-check-checklist-not-yaml":           overrideRefused,
		"scen-check-check-with-not-a-mapping":     overrideRefused,
		"scen-check-skip-not-boolean":             overrideRefused,
		"scen-check-failed-check-skips-questions": overrideChecks,
		"scen-check-term-defined-twice":           overrideChecks,
		"scen-check-terms-no-catalogue":           overrideChecks,
		"scen-check-fix-records-term":             overrideChecks,
		"scen-check-fix-records-concept":          overrideChecks,
		"scen-check-term-mapped-twice":            overrideChecks,
		"scen-check-step-binds-twice":             overrideChecks,
		"scen-check-definitions-correspond":       overrideChecks,
		"scen-check-definitions-unreadable":       overrideChecks,
		"scen-check-closure-no-catalogue":         overrideChecks,
		"concept-definitions-closure":             overrideChecks,
		"scen-check-fix-records-mapping":          overrideChecks,
		"scen-check-skipped-prompt":               overrideChecks,
		"us-refine-fix-unfixable-checklist":       overrideRefused,
		"codex-apply-us-refine":                   overrideChecks,
		"scen-check-prompt-names-its-tiers":       overrideTiers,
		"fix-loop-refine-feedback":                overrideFixLoop,
		"fix-loop-own-words":                      overrideFixLoop,
		"fix-loop-unanswered-eof":                 overrideFixLoop,
		"fix-loop-exit-terms":                     overrideFixLoop,
		"fix-loop-walks-exhausted-terms":          overrideFixLoop,
		"fix-loop-attempts-exhausted":             overrideFixLoop,
		"fix-loop-refix-stops":                    overrideFixLoop,
		"fix-loop-unsettled":                      overrideFixLoop,
	}

	fixtures := append(interfaceCheckFixtures(), conceptLinkFixtures()...)
	fixtures = append(fixtures, conceptWordFixtures()...)
	fixtures = append(fixtures, groupingOverrideFixtures()...)

	for _, fixture := range fixtures {
		allowed[fixture] = overrideChecks
	}

	return allowed
}

// interfaceCheckFixtures narrow scen check to its public-interface and
// concept-bindings checks and concept document closure (#239, #256, #259).
func interfaceCheckFixtures() []string {
	return []string{
		"concepts-valid", "concepts-catalogue-retired", "concepts-undefined-term", "concepts-unmapped-term",
		"concepts-bindings", "concepts-phrases-retired", "concepts-fix-binding", "concepts-fix-operation",
		"public-interface-missing", "public-interface-missing-document", "public-interface-malformed-document",
		"public-interface-valid", "public-interface-unsupported", "public-interface-fix",
	}
}

// TestNoUnsanctionedChecklistOverrides enforces the no-copied-checklists
// policy: any override file outside the allowlist fails.
func TestNoUnsanctionedChecklistOverrides(t *testing.T) {
	t.Parallel()

	for _, override := range discoverOverrides(t) {
		if allowedOverrideFixtures()[override.Fixture] == overrideUnsanctioned {
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
		purpose := allowedOverrideFixtures()[override.Fixture]
		if purpose == overrideRefused || purpose == overridePreserved {
			continue
		}

		prompts, loadErr := loadChecklistPrompts(override.Path, override.Stem)
		if loadErr != nil {
			t.Errorf("%s: loading override: %v", override.Fixture, loadErr)

			continue
		}

		assertOverrideCopiesShipped(t, uni, override, purpose, prompts)
	}
}

// assertOverrideCopiesShipped holds one walkable override to its purpose:
// empty only when sanctioned so, and every prompt but a sanctioned `D:` shipped.
func assertOverrideCopiesShipped(
	t *testing.T, uni *Universe, override overrideFile, purpose overridePurpose, prompts []UniversePrompt,
) {
	t.Helper()

	deliberatelyEmpty := purpose == overrideEmpty

	if len(prompts) == 0 && !deliberatelyEmpty {
		t.Errorf("%s: override has no active prompts — the fixture would walk nothing; "+
			"sanction it as overrideEmpty if that is the behaviour under test",
			override.Fixture)
	}

	if len(prompts) > 0 && deliberatelyEmpty {
		t.Errorf("%s: sanctioned as overrideEmpty but its override has %d prompt(s)",
			override.Fixture, len(prompts))
	}

	for _, prompt := range prompts {
		if prompt.Check && purpose == overrideChecks {
			continue
		}

		if purpose == overrideFixLoop {
			assertQuestionShipped(t, uni, override, prompt)

			continue
		}

		if uni.MatchOverride(override.Stem, prompt) == nil {
			t.Errorf("%s: override prompt %q does not match any shipped %s prompt",
				override.Fixture, prompt.Snippet, override.Stem)
		}
	}
}

// assertQuestionShipped holds a fix-loop override's prompt to a shipped
// question or check of the same checklist, its `F:` free to differ.
func assertQuestionShipped(t *testing.T, uni *Universe, override overrideFile, prompt UniversePrompt) {
	t.Helper()

	for _, shipped := range uni.Prompts {
		if shipped.Checklist == override.Stem && shipped.QHash == prompt.QHash {
			return
		}
	}

	t.Errorf("%s: override prompt %q asks no shipped %s question or check",
		override.Fixture, prompt.Snippet, override.Stem)
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

	data, err := disk.Read(
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
		// A fix-loop override may bound the loop (#168); no walk reads a refused one, and
		// scen-check-checklist-not-yaml's is not YAML (#171).
		purpose := allowedOverrideFixtures()[override.Fixture]
		if purpose == overrideFixLoop || purpose == overrideRefused || purpose == overridePreserved {
			continue
		}

		shipped := loadConfigBlock(t, filepath.Join(shippedChecklistsDir, override.Stem+".yaml"))
		overridden := loadConfigBlock(t, override.Path)

		normalizeGroupingConfig(t, override, shipped, overridden)

		if !reflect.DeepEqual(shipped.Config, overridden.Config) {
			t.Errorf("%s: config block diverges from shipped %s.yaml: shipped=%v override=%v",
				override.Fixture, override.Stem, shipped.Config, overridden.Config)
		}
	}
}

// loadConfigBlock reads just the config mapping of a checklist file.
func loadConfigBlock(t *testing.T, path string) configBlock {
	t.Helper()

	data, err := disk.Read(path)
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

func conceptLinkFixtures() []string {
	return []string{
		"concept-link-add-concept",
		"concept-link-ambiguous",
		"concept-link-ambiguous-binding",
		"concept-link-fix-binding",
		"concept-link-incomplete",
		"concept-link-judge-intent",
		"concept-link-llm-needs-intent",
		"concept-link-malformed",
		"concept-link-missing-answer",
		"concept-link-missing-binding",
		"concept-link-refine-before-binding",
		"concept-link-rewrite",
		"concept-link-shared-fix",
		"concept-link-threshold-eight",
		"concept-link-threshold-nine",
		"concept-link-unfixed",
		"concept-link-unquoted-confirmation",
		"concept-link-unrelated-unbound",
		"concept-link-widen",
		"concept-link-wrong-block",
	}
}

// Concept word fixtures isolate the link, word verdict and term-mapping handoff.
func conceptWordFixtures() []string {
	return []string{
		"concept-words-declare", "concept-words-reword", "concept-words-plain",
		"concept-words-scenario-fail", "concept-words-mapping", "concept-words-scenario-fix",
	}
}

// Omitted grouping_batch_texts is the engine default; only its batch Scenario changes it.
func normalizeGroupingConfig(t *testing.T, override overrideFile, shipped, overridden configBlock) {
	t.Helper()

	if override.Stem != "build-steps" || overridden.Config == nil {
		return
	}

	const (
		key           = "grouping_batch_texts"
		scenarioLimit = 4
	)
	if override.Fixture == groupingBatchesFixture && overridden.Config[key] != scenarioLimit {
		t.Errorf("%s: grouping batch limit must stay %d", override.Fixture, scenarioLimit)
	}

	if _, present := overridden.Config[key]; !present || override.Fixture == groupingBatchesFixture {
		overridden.Config[key] = shipped.Config[key]
	}
}

const groupingBatchesFixture = "concept-grouping-batches"

func groupingOverrideFixtures() []string {
	return []string{"concept-grouping-ambiguous", "concept-grouping-binding", groupingBatchesFixture}
}
