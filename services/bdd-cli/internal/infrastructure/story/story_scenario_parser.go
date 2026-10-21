package story

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const stepModifierMappingFields = 2

// ErrDeprecatedStoryFormat is returned when a story file carries the
// legacy `scenarios.test_scenarios[]` block rather than the canonical
// `story.acceptance_criteria[].steps` shape.
var ErrDeprecatedStoryFormat = errors.New(
	"story uses deprecated scenarios.test_scenarios format; us apply requires acceptance_criteria with embedded steps",
)

// ErrNoAcceptanceCriteria is returned when the story has no AC entries
// to apply.
var ErrNoAcceptanceCriteria = errors.New(
	"story has no acceptance_criteria entries to apply",
)

// ErrUnkeyableCriterionID is returned when an acceptance criterion's id is
// not `<NNN>-<slug>`: the id becomes the Scenario's Registry key (ADR 0014).
// Free of double quotes, because the refusal reaches stdout as a quoted slog value.
var ErrUnkeyableCriterionID = errors.New(
	"the id must be <NNN>-<slug>, three digits then a lower-case slug such as 001-unknown-id-refused, " +
		"because it becomes the Scenario's Registry key <story-id>-<ac-id>",
)

// criterionIDShape is `<NNN>-<slug>`: the part of a Registry key the author
// writes, so it survives reordering the Story's criteria.
var criterionIDShape = regexp.MustCompile(`^[0-9]{3}-[a-z0-9]+(-[a-z0-9]+)*$`)

// ErrMalformedStepModifier signals that a Gherkin step modifier
// (`{and: ...}` / `{but: ...}`) has the wrong YAML shape.
var ErrMalformedStepModifier = errors.New("malformed step modifier")

// ErrUnexpectedStepKind signals that a step entry was neither a scalar
// nor a one-key mapping.
var ErrUnexpectedStepKind = errors.New("unexpected step kind")

// StoryScenarioParser reads a refined story file and emits one
// ScenarioApplyData per acceptance criterion. Stories that still use
// the deprecated `scenarios.test_scenarios[]` block are rejected.
type StoryScenarioParser struct {
	storiesDir string
}

// NewStoryScenarioParser builds a parser rooted at the configured
// stories directory.
func NewStoryScenarioParser(cfg *config.ViperConfig) *StoryScenarioParser {
	return &StoryScenarioParser{
		storiesDir: cfg.GetString("paths.stories_dir"),
	}
}

// rawACStep mirrors the YAML shape of an AC's individual step entry.
// Each list item carries one of given/when/then with a list of step
// statements; trailing modifiers appear as `{and: "..."}` items.
type rawACStep struct {
	Given []rawStatement `yaml:"given,omitempty"`
	When  []rawStatement `yaml:"when,omitempty"`
	Then  []rawStatement `yaml:"then,omitempty"`
}

// rawStatement decodes either a plain string or an `{and|but: "..."}`
// modifier entry. The flattened text becomes a single MergedSteps line.
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

type rawAC struct {
	ID          string      `yaml:"id"`
	Description string      `yaml:"description"`
	Steps       []rawACStep `yaml:"steps"`
}

type rawStoryFile struct {
	Story struct {
		ID                 string  `yaml:"id"`
		AcceptanceCriteria []rawAC `yaml:"acceptance_criteria"`
	} `yaml:"story"`

	// Scenarios captures only enough of the legacy block to detect
	// it; we never read the entries themselves. A non-empty
	// `test_scenarios` list signals the deprecated 3.x format.
	Scenarios struct {
		TestScenarios []yaml.Node `yaml:"test_scenarios"`
	} `yaml:"scenarios"`
}

// ParseStoryScenarios reads the Story file FindStoryFile resolves
// storyNumber to, validates the canonical AC-with-steps shape, and returns
// one ScenarioApplyData per AC plus the resolved story path.
func (p *StoryScenarioParser) ParseStoryScenarios(
	storyNumber string,
	requirementsScratchPath string,
) ([]*template.ScenarioApplyData, string, error) {
	storyFile, err := FindStoryFile(p.storiesDir, storyNumber)
	if err != nil {
		return nil, "", err
	}

	storyPath := storyFile.Path

	var raw rawStoryFile

	err = yaml.Unmarshal(storyFile.Data, &raw)
	if err != nil {
		return nil, storyPath, pkgerrors.ErrParseStoryYAMLFailed(storyPath, err)
	}

	if len(raw.Scenarios.TestScenarios) > 0 {
		return nil, storyPath, fmt.Errorf("%s: %w", storyPath, ErrDeprecatedStoryFormat)
	}

	if len(raw.Story.AcceptanceCriteria) == 0 {
		return nil, storyPath, fmt.Errorf("%s: %w", storyPath, ErrNoAcceptanceCriteria)
	}

	// FindStoryFile picked this file because it declares storyNumber.
	storyID := raw.Story.ID

	scenarios := make([]*template.ScenarioApplyData, 0, len(raw.Story.AcceptanceCriteria))

	for index, criterion := range raw.Story.AcceptanceCriteria {
		if !criterionIDShape.MatchString(criterion.ID) {
			return nil, storyPath, fmt.Errorf("%s: acceptance criterion %d has id %s: %w",
				storyPath, index+1, criterion.ID, ErrUnkeyableCriterionID)
		}

		given, when, then := flattenACSteps(criterion.Steps)
		// The lineage id is the Scenario's Registry key, `<story-id>-<ac-id>`
		// (ADR 0014): never the criterion's position, so reordering keeps it.
		lineageID := storyID + "-" + criterion.ID

		scenarios = append(scenarios, template.NewScenarioApplyData(
			storyID,
			storyPath,
			criterion.ID,
			lineageID,
			criterion.Description,
			given,
			when,
			then,
			requirementsScratchPath,
		))
	}

	slog.Info("Parsed story scenarios for apply",
		"story", storyID,
		"file", storyPath,
		"count", len(scenarios),
	)

	return scenarios, storyPath, nil
}

// flattenACSteps walks an AC's steps list and returns the Given/When/Then
// arrays as flat strings. Modifier entries (`and:`/`but:`) are prefixed
// with their keyword so downstream display matches what a reader sees.
func flattenACSteps(steps []rawACStep) ([]string, []string, []string) {
	given := make([]string, 0)
	when := make([]string, 0)
	then := make([]string, 0)

	for _, step := range steps {
		for _, statement := range step.Given {
			given = append(given, statement.value)
		}

		for _, statement := range step.When {
			when = append(when, statement.value)
		}

		for _, statement := range step.Then {
			then = append(then, statement.value)
		}
	}

	return given, when, then
}
