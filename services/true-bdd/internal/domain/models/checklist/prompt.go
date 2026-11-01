package checklist

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Why a prompt cannot be walked whatever checks the engine holds (ADR
// 0015). Free of regex metacharacters: fixtures pin them with `stdout matches`.
var (
	ErrPromptHasBothKinds = errors.New(
		"carries both D: and Q:, but a built-in check or a model answers a prompt, never both")
	ErrPromptHasNoKind = errors.New("carries neither D: nor Q:, so nothing would answer it")
	ErrModelOnCheck    = errors.New("carries model:, but no model answers a built-in check")
	ErrSkipNotBoolean  = errors.New("skip: takes true or false")
	errWithNotMapping  = errors.New("with: takes a mapping of check parameters")
)

// Prompt represents a single validation prompt: a model answers its
// Question (`Q:`), or the built-in check its Check names (`D:`) does.
type Prompt struct {
	Question    string         // The validation question (Q in YAML)
	Check       string         // The built-in check answering it instead (D in YAML, trimmed)
	With        map[string]any // The check's parameters (with in YAML)
	Rationale   string         // Why this matters
	Skip        bool           // Parks the prompt, left out of the walk (skip: true in YAML)
	Docs        []string       // Document keys to inject into prompt
	FixTemplate string         // Template for generating fix prompt when validation fails (F in YAML)
	// Model tier overrides for this prompt alone, each shadowing the
	// checklist's EngineBlock. Empty means "inherit".
	Model      string // Tier for the validation turn (model in YAML)
	FixModel   string // Tier for the fix-generation turn (fix_model in YAML)
	ApplyModel string // Tier for the fix-application turn (apply_model in YAML)
	// skipValue is a `skip:` that is not a boolean, kept for the refusal:
	// YAML reads "false" and yes as strings, and either reading is a guess.
	skipValue any
}

// UnmarshalYAML implements custom YAML unmarshaling for Prompt.
// This handles the uppercase Q field from the YAML format.
func (p *Prompt) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]interface{}

	err := node.Decode(&raw)
	if err != nil {
		return fmt.Errorf("failed to decode prompt: %w", err)
	}

	p.Question = getStringFromMap(raw, "Q")
	p.Check = strings.TrimSpace(getStringFromMap(raw, "D"))

	err = p.decodeWith(raw)
	if err != nil {
		return err
	}

	p.Rationale = getStringFromMap(raw, "rationale")
	p.decodeSkip(raw)
	p.Docs = getStringSliceFromMap(raw, "docs")
	p.FixTemplate = getStringFromMap(raw, "F")
	p.Model = getStringFromMap(raw, "model")
	p.FixModel = getStringFromMap(raw, "fix_model")
	p.ApplyModel = getStringFromMap(raw, "apply_model")

	return nil
}

func getStringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}

	return ""
}

func getStringSliceFromMap(data map[string]interface{}, key string) []string {
	if val, ok := data[key]; ok {
		if sliceVal, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(sliceVal))
			for _, item := range sliceVal {
				if strItem, ok := item.(string); ok {
					result = append(result, strItem)
				}
			}

			return result
		}
	}

	return nil
}

// ShouldSkip reports whether the prompt is parked by `skip: true`; a
// `skip:` that is not a boolean parks nothing, and ShapeError refuses it.
func (p *Prompt) ShouldSkip() bool {
	return p.Skip
}

// IsCheck reports whether a built-in check answers the prompt (`D:`)
// rather than a model (`Q:`).
func (p *Prompt) IsCheck() bool {
	return p.Check != ""
}

// ShapeError reports why the prompt cannot be walked whatever checks the
// engine holds, or nil; which checks exist is the walking command's call.
func (p *Prompt) ShapeError() error {
	switch {
	case p.skipValue != nil:
		return fmt.Errorf("%s carries skip: %s, but %w", p.Label(), describeValue(p.skipValue), ErrSkipNotBoolean)
	case p.IsCheck() && p.asksModel():
		return ErrPromptHasBothKinds
	case !p.IsCheck() && !p.asksModel():
		return ErrPromptHasNoKind
	case p.IsCheck() && p.Model != "":
		return fmt.Errorf("%s %w", p.Label(), ErrModelOnCheck)
	}

	return p.conceptShapeError()
}

// Label names the prompt in one line: `D: <check>` for a check, else the
// first non-blank line of its question.
func (p *Prompt) Label() string {
	if p.IsCheck() {
		return "D: " + p.Check
	}

	for line := range strings.SplitSeq(p.Question, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// asksModel reports whether the prompt carries a `Q:` question.
func (p *Prompt) asksModel() bool {
	return strings.TrimSpace(p.Question) != ""
}

// decodeSkip reads `skip:`: absent or null walks the prompt, a boolean
// says whether to park it, and anything else is kept for ShapeError.
func (p *Prompt) decodeSkip(raw map[string]interface{}) {
	val, present := raw["skip"]
	if !present || val == nil {
		return
	}

	flag, isBool := val.(bool)
	if !isBool {
		p.skipValue = val

		return
	}

	p.Skip = flag
}

// describeValue renders a YAML value for a refusal, quoting a string so
// `"false"` reads as the string it is.
func describeValue(val any) string {
	if text, isString := val.(string); isString {
		return fmt.Sprintf("%q", text)
	}

	return fmt.Sprintf("%v", val)
}

// decodeWith reads the check parameters: absent leaves With nil, and any
// shape but a mapping is refused, since dropping it would run the check unparameterised.
func (p *Prompt) decodeWith(raw map[string]interface{}) error {
	val, present := raw["with"]
	if !present || val == nil {
		return nil
	}

	mapping, isMapping := val.(map[string]interface{})
	if !isMapping {
		return fmt.Errorf("%s carries a with: that is %s, but %w", p.Label(), describeShape(val), errWithNotMapping)
	}

	p.With = mapping

	return nil
}

// describeShape names a YAML value's kind as a checklist's author wrote
// it, never the Go type it decoded into.
func describeShape(val any) string {
	switch val.(type) {
	case []interface{}:
		return "a list"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case int, int64, uint64, float64:
		return "a number"
	default:
		return "not a mapping"
	}
}

// PromptWithContext holds a prompt with its section context for evaluation.
type PromptWithContext struct {
	SectionID     string       // e.g., "template", "invest", "dependencies_risks"
	SectionName   string       // e.g., "Template", "INVEST", "Dependencies & Risks"
	CriterionID   string       // e.g., "who", "valuable" (empty if no criterion)
	CriterionName string       // e.g., "Who", "Valuable" (empty if no criterion)
	DefaultDocs   []string     // Default docs from checklist root level
	DefaultModels *EngineBlock // Default model tiers from the checklist `engine:` block
	// Index is the 1-based position in the checklist, in file order: it
	// names the Cell's artifacts and orders the report, though `D:` walks first.
	Index  int
	Prompt Prompt
}

// GetEffectiveDocs returns the docs to use (prompt-specific or defaults).
func (p *PromptWithContext) GetEffectiveDocs() []string {
	if len(p.Prompt.Docs) > 0 {
		return p.Prompt.Docs
	}

	return p.DefaultDocs
}

// EffectiveModelTier returns the tier for the validation turn: the
// prompt's own `model:`, else the checklist's `prompt_model:`, else ""
// meaning `engine.default_prompt_model`.
func (p *PromptWithContext) EffectiveModelTier() string {
	if p.Prompt.Model != "" {
		return p.Prompt.Model
	}

	if p.DefaultModels != nil {
		return p.DefaultModels.PromptModel
	}

	return ""
}

// EffectiveFixTier returns the tier for the fix-generation turn,
// resolved prompt → checklist → `engine.default_fix_model`.
func (p *PromptWithContext) EffectiveFixTier() string {
	if p.Prompt.FixModel != "" {
		return p.Prompt.FixModel
	}

	if p.DefaultModels != nil {
		return p.DefaultModels.FixModel
	}

	return ""
}

// EffectiveApplyTier returns the tier for the fix-application turn,
// resolved prompt → checklist → `engine.default_apply_model`.
func (p *PromptWithContext) EffectiveApplyTier() string {
	if p.Prompt.ApplyModel != "" {
		return p.Prompt.ApplyModel
	}

	if p.DefaultModels != nil {
		return p.DefaultModels.ApplyModel
	}

	return ""
}

// GetFullSectionPath returns the full section path for display.
func (p *PromptWithContext) GetFullSectionPath() string {
	if p.CriterionID != "" {
		return p.SectionID + "/" + p.CriterionID
	}

	return p.SectionID
}
