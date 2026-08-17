package registry

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

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
// its text.
//
// Steps below renders the same lines flattened into display strings,
// which is what the prompt templates show. This is the form the test
// generator needs: a generated file calls `s.And(...)` and `s.But(...)`
// as separate methods, so "and" has to still be a keyword rather than
// two words at the front of a sentence.
type Statement struct {
	Keyword string
	Text    string
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

// FormatSteps renders Given / When / Then for display in the
// build-tests prompt templates. Same shape as
// template.ScenarioApplyData.FormatSteps so the .tpl files can read
// `{{.Subject.FormatSteps}}` interchangeably.
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
		// Exactly one pair, matching bddgo's `!= 2`. A `< 2` check
		// ACCEPTS `{and: A, but: B}` and reads only the first pair, so
		// `build tests` would render a test missing the second clause and
		// call the scenario converged, while bddgo refuses the whole
		// registry — the laxer reader silently dropping behaviour is the
		// worse half of that disagreement.
		if len(node.Content) != stepModifierMappingFields {
			return fmt.Errorf("%w near line %d", ErrMalformedStepModifier, node.Line)
		}

		key := node.Content[0].Value

		// The VALUE has to be a scalar too. `node.Value` is empty for a
		// sequence or a mapping, so `- and: [A, B]` would otherwise set the
		// step's text to "" — a step that renders as `s.And("")`, binds to
		// nothing, and reports as a step the author wrote rather than as the
		// malformed one it is. Same class as the arity check above: the
		// silent drop is worse than the refusal.
		if node.Content[1].Kind != yaml.ScalarNode {
			return fmt.Errorf("%w: %q takes a scalar, got kind=%d near line %d",
				ErrMalformedStepModifier, key, node.Content[1].Kind, node.Content[1].Line)
		}

		val := node.Content[1].Value

		// `and` and `but` and nothing else, matching bddgo's own decoder
		// (tests/libraries/bddgo/registry.go). Two readers of one
		// document that disagree about what a step IS would eventually
		// disagree about what it means — and here the laxer one would
		// render `s.Foo(...)` into a generated file, which is valid Go
		// that does not compile. An empty key is refused by the same
		// check, rather than reaching a slice index that panics.
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
// intact. The first step of a block takes the block's keyword and the
// rest default to And, which is how the same lines read on a feature
// file — and the same rule bddgo's own loader applies, so the generated
// file and the registry classify a step identically.
func (r rawMergedSteps) statements() []Statement {
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
			out = append(out, Statement{
				Keyword: statementKeyword(step, block.keyword, index),
				Text:    step.text,
			})
		}
	}

	return out
}

// statementKeyword resolves one step's keyword. The BLOCK's keyword wins
// at index 0 even when the step spelled a modifier for itself, and the
// same rule lives in bddgo's stepKeyword.
//
// It has to win, not merely read better. A generated file states each
// step by calling the method its keyword names, and And/But inherit the
// block from the call above them — so a block emitted as `s.And(…)`
// first would inherit the PREVIOUS block, and a step the registry
// classified against then: would be re-classified against when: at run
// time. `And` as the opening line of a block is not Gherkin anyway.
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

// Load reads the YAML registry at `path`, flattens the `scenarios:` map
// into a slice sorted by id, and returns one RegistryScenario per
// entry. YAML maps are unordered in Go, so the deterministic sort makes
// stdout output and tmp-file naming stable across runs.
func (l *RegistryLoader) Load(path string) ([]*RegistryScenario, error) {
	data, err := os.ReadFile(path)
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

	for id, entry := range raw.Scenarios {
		scenarios = append(scenarios, &RegistryScenario{
			ID:          id,
			Description: entry.Description,
			Service:     entry.Service,
			Path:        entry.Path,
			Requirement: entry.Requirement,
			Steps: template.MergedSteps{
				Given: flattenStatements(entry.MergedSteps.Given),
				When:  flattenStatements(entry.MergedSteps.When),
				Then:  flattenStatements(entry.MergedSteps.Then),
			},
			Statements:  entry.MergedSteps.statements(),
			UserStories: entry.UserStories,
		})
	}

	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].ID < scenarios[j].ID
	})

	slog.Info("Loaded requirements registry",
		"file", path,
		"count", len(scenarios),
	)

	return scenarios, nil
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
