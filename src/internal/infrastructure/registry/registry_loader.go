package registry

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/infrastructure/template"
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

// RegistryScenario is one entry in `docs/requirements.yaml#/scenarios`
// after lineage and Given/When/Then steps are materialized.
type RegistryScenario struct {
	ID          string
	Description string
	Service     string
	Requirement string
	Steps       template.MergedSteps
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
	value string
}

// UnmarshalYAML supports both scalar steps ("a user does X") and
// modifier steps (`{and: "..."}` / `{but: "..."}`).
func (s *rawStatement) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.value = node.Value

		return nil
	}

	if node.Kind == yaml.MappingNode {
		if len(node.Content) < stepModifierMappingFields {
			return fmt.Errorf("%w near line %d", ErrMalformedStepModifier, node.Line)
		}

		key := node.Content[0].Value
		val := node.Content[1].Value
		s.value = key + " " + val

		return nil
	}

	return fmt.Errorf("%w (kind=%d) near line %d", ErrUnexpectedStepKind, node.Kind, node.Line)
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
	Requirement string         `yaml:"requirement,omitempty"`
	UserStories []UserStoryRef `yaml:"user_stories,omitempty"`
	MergedSteps rawMergedSteps `yaml:"merged_steps"`
}

// rawRegistry mirrors the top-level shape of docs/requirements.yaml.
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
			Requirement: entry.Requirement,
			Steps: template.MergedSteps{
				Given: flattenStatements(entry.MergedSteps.Given),
				When:  flattenStatements(entry.MergedSteps.When),
				Then:  flattenStatements(entry.MergedSteps.Then),
			},
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
