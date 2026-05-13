package checklist

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Prompt represents a single validation prompt.
type Prompt struct {
	Question     string   // The validation question (Q in YAML)
	Rationale    string   // Why this matters
	Skip         string   // If set, skip this prompt
	ActionIfYes  string   // Action if answer is yes
	ActionIfFail string   // Action if validation fails
	Docs         []string // Document keys to inject into prompt
	FixTemplate  string   // Template for generating fix prompt when validation fails (F in YAML)
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
	p.Rationale = getStringFromMap(raw, "rationale")
	p.Skip = getStringFromMap(raw, "skip")
	p.ActionIfYes = getStringFromMap(raw, "action_if_yes")
	p.ActionIfFail = getStringFromMap(raw, "action_if_fail")
	p.Docs = getStringSliceFromMap(raw, "docs")
	p.FixTemplate = getStringFromMap(raw, "F")

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

// ShouldSkip returns true if this prompt should be skipped.
func (p *Prompt) ShouldSkip() bool {
	return p.Skip != ""
}

// PromptWithContext holds a prompt with its section context for evaluation.
type PromptWithContext struct {
	SectionID     string   // e.g., "template", "invest", "dependencies_risks"
	SectionName   string   // e.g., "Template", "INVEST", "Dependencies & Risks"
	CriterionID   string   // e.g., "who", "valuable" (empty if no criterion)
	CriterionName string   // e.g., "Who", "Valuable" (empty if no criterion)
	DefaultDocs   []string // Default docs from checklist root level
	Prompt        Prompt
}

// GetEffectiveDocs returns the docs to use (prompt-specific or defaults).
func (p *PromptWithContext) GetEffectiveDocs() []string {
	if len(p.Prompt.Docs) > 0 {
		return p.Prompt.Docs
	}

	return p.DefaultDocs
}

// GetFullSectionPath returns the full section path for display.
func (p *PromptWithContext) GetFullSectionPath() string {
	if p.CriterionID != "" {
		return p.SectionID + "/" + p.CriterionID
	}

	return p.SectionID
}
