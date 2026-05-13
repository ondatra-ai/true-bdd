package story

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"bdd-cli/src/internal/infrastructure/config"
	"bdd-cli/src/internal/infrastructure/template"
	pkgerrors "bdd-cli/src/internal/pkg/errors"
)

const (
	lineageScenarioIDFormat   = "%s-%03d"
	stepModifierMappingFields = 2
)

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

// ParseStoryScenarios resolves docs/stories/<storyNumber>-*.yaml,
// validates it uses the canonical AC-with-steps shape, and emits one
// ScenarioApplyData per AC. The returned slice carries the supplied
// scratch path on every entry so downstream prompts can reference it.
// The resolved story file path is returned for downstream use
// (lineage, error messages).
func (p *StoryScenarioParser) ParseStoryScenarios(
	storyNumber string,
	requirementsScratchPath string,
) ([]*template.ScenarioApplyData, string, error) {
	storyPath, err := p.resolveStoryPath(storyNumber)
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(storyPath)
	if err != nil {
		return nil, storyPath, pkgerrors.ErrReadStoryFileFailed(storyPath, err)
	}

	var raw rawStoryFile

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, storyPath, pkgerrors.ErrParseStoryYAMLFailed(storyPath, err)
	}

	if len(raw.Scenarios.TestScenarios) > 0 {
		return nil, storyPath, fmt.Errorf("%s: %w", storyPath, ErrDeprecatedStoryFormat)
	}

	if len(raw.Story.AcceptanceCriteria) == 0 {
		return nil, storyPath, fmt.Errorf("%s: %w", storyPath, ErrNoAcceptanceCriteria)
	}

	storyID := raw.Story.ID
	if storyID == "" {
		storyID = storyNumber
	}

	scenarios := make([]*template.ScenarioApplyData, 0, len(raw.Story.AcceptanceCriteria))

	for index, criterion := range raw.Story.AcceptanceCriteria {
		given, when, then := flattenACSteps(criterion.Steps)
		lineageID := fmt.Sprintf(lineageScenarioIDFormat, storyID, index+1)

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

// resolveStoryPath finds docs/stories/<storyNumber>-*.yaml. Mirrors the
// resolution logic in StoryLoader to keep behavior consistent.
func (p *StoryScenarioParser) resolveStoryPath(storyNumber string) (string, error) {
	pattern := filepath.Join(p.storiesDir, storyNumber+"-*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", pkgerrors.ErrFindStoryFileFailed(err)
	}

	if len(matches) == 0 {
		return "", pkgerrors.ErrStoryFileNotFoundError(storyNumber, p.storiesDir, storyNumber)
	}

	if len(matches) > 1 {
		return "", pkgerrors.ErrMultipleStoryFilesError(storyNumber, matches)
	}

	return matches[0], nil
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
