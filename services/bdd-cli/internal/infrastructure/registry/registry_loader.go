package registry

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

const stepModifierMappingFields = 2

// ErrNoScenarios signals that the requirements registry has no
// `scenarios:` map to walk.
var ErrNoScenarios = errors.New(
	"requirements registry has no scenarios to walk",
)

// ErrMalformedStepModifier signals that a Gherkin step modifier
// (`{and: ...}` / `{but: ...}`) has the wrong YAML shape. Mirrors the
// story_scenario_parser error so registry-side messages read the same.
var ErrMalformedStepModifier = errors.New("malformed step modifier")

// ErrUnexpectedStepKind signals that a step entry was neither a scalar
// nor a one-key mapping.
var ErrUnexpectedStepKind = errors.New("unexpected step kind")

// UserStoryRef is one entry from a scenario's `user_stories[]` lineage
// list. Only the fields the build-tests templates need are decoded.
type UserStoryRef struct {
	Story      string `yaml:"story"`
	ScenarioID string `yaml:"scenario_id"`
	MergeDate  string `yaml:"merge_date,omitempty"`
}

// Gherkin keywords a statement can carry.
const (
	KeywordGiven = "Given"
	KeywordWhen  = "When"
	KeywordThen  = "Then"
	KeywordAnd   = "And"
	KeywordBut   = "But"
)

// Statement is one Given/When/Then line with its keyword kept apart from
// its text: the generated test file calls `s.And(...)`/`s.But(...)` as
// separate methods, so "and"/"but" must stay a keyword, not plain text.
type Statement struct {
	Keyword string
	// Mode says who executes the step, resolved against the BLOCK's
	// keyword. Text keeps any `llm:`/`judge:` prefix verbatim: the
	// generated test quotes Text and bddgo strips the prefix there.
	Mode StepMode
	Text string
}

// RegistryScenario is one entry in `docs/scenarios.yaml#/scenarios`
// after lineage and Given/When/Then steps are materialized.
type RegistryScenario struct {
	ID          string
	Description string
	Service     string
	// Path is the generated test file this scenario is written into,
	// repo-relative. Required; `build tests` refuses a registry missing
	// it rather than choosing a file on the author's behalf.
	Path        string
	Requirement string
	Steps       template.MergedSteps
	Statements  []Statement
	UserStories []UserStoryRef
}

// FormatSteps renders Given/When/Then for the build-tests prompt templates.
// Same shape as template.ScenarioApplyData.FormatSteps so .tpl files can
// read `{{.Subject.FormatSteps}}` interchangeably.
func (s *RegistryScenario) FormatSteps() string {
	var result strings.Builder

	if len(s.Steps.Given) > 0 {
		result.WriteString("Given:\n")

		for _, step := range s.Steps.Given {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	if len(s.Steps.When) > 0 {
		result.WriteString("When:\n")

		for _, step := range s.Steps.When {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	if len(s.Steps.Then) > 0 {
		result.WriteString("Then:\n")

		for _, step := range s.Steps.Then {
			fmt.Fprintf(&result, "  - %s\n", step)
		}
	}

	return result.String()
}

// rawStatement decodes either a plain string or an `{and|but: "..."}`
// modifier entry. Mirrors the story-side decoder so registry YAML reads
// identically. One extra caller doesn't justify a shared package.
type rawStatement struct {
	// value is the display form the prompt templates show: a modifier
	// entry reads "and <text>", the way it reads in the document.
	value string
	// keyword is the modifier a step spelled for itself, if any, and
	// text is what it said. Kept apart from value because the generator
	// needs the two separately and the templates need them joined.
	keyword string
	text    string
}

// UnmarshalYAML supports both scalar steps ("a user does X") and
// modifier steps (`{and: "..."}` / `{but: "..."}`).
func (s *rawStatement) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.value = node.Value
		s.text = node.Value

		return nil
	}

	if node.Kind == yaml.MappingNode {
		// Exactly one pair, matching bddgo's `!= 2`. A `< 2` check would ACCEPT
		// `{and: A, but: B}`, silently drop the second clause, and call the
		// scenario converged — while bddgo refuses the whole registry for it.
		if len(node.Content) != stepModifierMappingFields {
			return fmt.Errorf("%w near line %d", ErrMalformedStepModifier, node.Line)
		}

		key := node.Content[0].Value

		// The VALUE must be a scalar too: `node.Value` is empty for a sequence
		// or mapping, so `- and: [A, B]` would otherwise set text to "" and
		// render as `s.And("")` — a malformed step disguised as one the author wrote.
		if node.Content[1].Kind != yaml.ScalarNode {
			return fmt.Errorf("%w: %q takes a scalar, got kind=%d near line %d",
				ErrMalformedStepModifier, key, node.Content[1].Kind, node.Content[1].Line)
		}

		val := node.Content[1].Value

		// `and`/`but` only, matching bddgo's own decoder (registry.go): any
		// other key would render `s.Foo(...)` into a generated file — valid
		// Go that doesn't compile — and an empty key is refused here rather than panicking later.
		keyword, known := modifierKeyword(key)
		if !known {
			return fmt.Errorf("%w: got %q near line %d",
				ErrMalformedStepModifier, key, node.Line)
		}

		s.value = key + " " + val
		s.keyword = keyword
		s.text = val

		return nil
	}

	return fmt.Errorf("%w (kind=%d) near line %d", ErrUnexpectedStepKind, node.Kind, node.Line)
}

// modifierKeyword maps a step mapping's key onto its Gherkin keyword,
// reporting whether it is one the registry allows at all.
func modifierKeyword(key string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "and":
		return KeywordAnd, true
	case "but":
		return KeywordBut, true
	default:
		return "", false
	}
}

// statements renders the three blocks into one ordered list, keywords
// intact: the first step takes the block's keyword, the rest default to
// And — the same rule bddgo's loader applies, so both classify identically.
func (r rawMergedSteps) statements() ([]Statement, error) {
	out := make([]Statement, 0, len(r.Given)+len(r.When)+len(r.Then))

	blocks := []struct {
		keyword string
		steps   []rawStatement
	}{
		{KeywordGiven, r.Given},
		{KeywordWhen, r.When},
		{KeywordThen, r.Then},
	}

	for _, block := range blocks {
		for index, step := range block.steps {
			mode, err := statementMode(step.text, block.keyword)
			if err != nil {
				return nil, err
			}

			out = append(out, Statement{
				Keyword: statementKeyword(step, block.keyword, index),
				Mode:    mode,
				Text:    step.text,
			})
		}
	}

	return out, nil
}

// statementKeyword resolves one step's keyword: the BLOCK's keyword MUST
// win at index 0 (same rule as bddgo's stepKeyword) — a self-spelled
// modifier winning there would emit `s.And(…)` first, inheriting the PREVIOUS block and reclassifying it at runtime.
func statementKeyword(step rawStatement, blockKeyword string, index int) string {
	if index == 0 {
		return blockKeyword
	}

	if step.keyword != "" {
		return step.keyword
	}

	return KeywordAnd
}

// rawMergedSteps mirrors the `merged_steps:` block of a registry entry.
type rawMergedSteps struct {
	Given []rawStatement `yaml:"given,omitempty"`
	When  []rawStatement `yaml:"when,omitempty"`
	Then  []rawStatement `yaml:"then,omitempty"`
}

// rawScenario mirrors one value under `scenarios:` in the registry YAML.
type rawScenario struct {
	Description string         `yaml:"description"`
	Service     string         `yaml:"service"`
	Path        string         `yaml:"path,omitempty"`
	Requirement string         `yaml:"requirement,omitempty"`
	UserStories []UserStoryRef `yaml:"user_stories,omitempty"`
	MergedSteps rawMergedSteps `yaml:"merged_steps"`
}

// rawRegistry mirrors the top-level shape of docs/scenarios.yaml.
type rawRegistry struct {
	Scenarios map[string]rawScenario `yaml:"scenarios"`
}

// RegistryLoader reads the requirements registry into a sorted slice of
// RegistryScenario. Zero-value safe; no configuration required.
type RegistryLoader struct{}

// NewRegistryLoader builds a RegistryLoader.
func NewRegistryLoader() *RegistryLoader {
	return &RegistryLoader{}
}

// Load reads the YAML registry at `path`, flattens `scenarios:` into a
// slice sorted by id, one RegistryScenario per entry. The sort matters:
// YAML maps are unordered in Go, so output and naming would otherwise vary across runs.
func (l *RegistryLoader) Load(path string) ([]*RegistryScenario, error) {
	data, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read requirements registry %s: %w", path, err)
	}

	var raw rawRegistry

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse requirements registry %s: %w", path, err)
	}

	if len(raw.Scenarios) == 0 {
		return nil, fmt.Errorf("%s: %w", path, ErrNoScenarios)
	}

	scenarios := make([]*RegistryScenario, 0, len(raw.Scenarios))

	for _, id := range sortedIDs(raw.Scenarios) {
		scenario, scErr := newScenario(id, raw.Scenarios[id])
		if scErr != nil {
			return nil, fmt.Errorf("%s: %s: %w", path, id, scErr)
		}

		scenarios = append(scenarios, scenario)
	}

	slog.Info("Loaded requirements registry",
		"file", path,
		"count", len(scenarios),
	)

	return scenarios, nil
}

// sortedIDs orders the registry's ids. YAML maps are unordered in Go, so
// without this both the output order and WHICH scenario a per-scenario
// refusal names would vary run to run.
func sortedIDs(entries map[string]rawScenario) []string {
	ids := make([]string, 0, len(entries))

	for id := range entries {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	return ids
}

// newScenario materializes one registry entry.
func newScenario(scenarioID string, entry rawScenario) (*RegistryScenario, error) {
	statements, err := entry.MergedSteps.statements()
	if err != nil {
		return nil, err
	}

	return &RegistryScenario{
		ID:          scenarioID,
		Description: entry.Description,
		Service:     entry.Service,
		Path:        entry.Path,
		Requirement: entry.Requirement,
		Steps: template.MergedSteps{
			Given: flattenStatements(entry.MergedSteps.Given),
			When:  flattenStatements(entry.MergedSteps.When),
			Then:  flattenStatements(entry.MergedSteps.Then),
		},
		Statements:  statements,
		UserStories: entry.UserStories,
	}, nil
}

// flattenStatements turns []rawStatement into a flat []string.
func flattenStatements(raw []rawStatement) []string {
	out := make([]string, 0, len(raw))

	for _, statement := range raw {
		out = append(out, statement.value)
	}

	return out
}

// Subject is the GetSubject implementation for build-tests scenarios.
// Returns (id, description) for the report-builder header and tmp-file
// naming.
func Subject(item *RegistryScenario) (string, string) {
	return item.ID, item.Description
}
