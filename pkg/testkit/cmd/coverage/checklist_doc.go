package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// checklistDoc mirrors the production checklist model closely enough
// for flattening: default docs plus ordered sections of prompts.
type checklistDoc struct {
	DefaultDocs []string     `yaml:"default_docs"`
	Sections    []sectionDoc `yaml:"sections"`
}

// sectionDoc is one checklist section.
type sectionDoc struct {
	ID                string      `yaml:"id"`
	ValidationPrompts []promptDoc `yaml:"validation_prompts"`
}

// promptDoc is one validation prompt, decoded with production parity
// (map-based unmarshal accepts only string values — see TestSkipParity
// in universe_test.go for the skip: false quirk this preserves).
type promptDoc struct {
	Question    string
	Check       string         // the built-in check a `D:` prompt names
	With        map[string]any // that check's parameters
	Rationale   string
	Skip        string
	FixTemplate string
	Docs        []string
}

// errWithNotMapping mirrors the production refusal of a `with:` that is
// not a mapping of check parameters.
var errWithNotMapping = errors.New("with: takes a mapping of check parameters")

// UnmarshalYAML implements the production map-based prompt decoding.
func (p *promptDoc) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]interface{}

	err := node.Decode(&raw)
	if err != nil {
		return fmt.Errorf("decoding prompt: %w", err)
	}

	p.Question = stringFromMap(raw, "Q")
	p.Check = stringFromMap(raw, "D")

	with, present := raw["with"]
	if present && with != nil {
		mapping, isMapping := with.(map[string]interface{})
		if !isMapping {
			return fmt.Errorf("%w, got %T", errWithNotMapping, with)
		}

		p.With = mapping
	}

	p.Rationale = stringFromMap(raw, "rationale")
	p.Skip = stringFromMap(raw, "skip")
	p.FixTemplate = stringFromMap(raw, "F")
	p.Docs = stringSliceFromMap(raw, "docs")

	return nil
}

// isCheck reports whether a built-in check answers the prompt (`D:`),
// the production Prompt.IsCheck rule.
func (p *promptDoc) isCheck() bool {
	return strings.TrimSpace(p.Check) != ""
}

// identityText is what a prompt is censused by: its question, or for a
// check its name and parameters, since a `D:` prompt asks no question.
func (p *promptDoc) identityText() string {
	if !p.isCheck() {
		return p.Question
	}

	identity := "D: " + strings.TrimSpace(p.Check)

	if len(p.With) > 0 {
		// json.Marshal sorts map keys, so equal parameters encode equally.
		encoded, err := json.Marshal(p.With)
		if err == nil {
			identity += " with " + string(encoded)
		}
	}

	return identity
}

// parseChecklistDoc unmarshals a checklist YAML document.
func parseChecklistDoc(data []byte) (*checklistDoc, error) {
	var doc checklistDoc

	err := yaml.Unmarshal(data, &doc)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling checklist: %w", err)
	}

	return &doc, nil
}

// stringFromMap extracts a string value, ignoring non-string types —
// parity with the production getStringFromMap.
func stringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if s, isStr := val.(string); isStr {
			return s
		}
	}

	return ""
}

// stringSliceFromMap extracts a string list, ignoring non-string items —
// parity with the production getStringSliceFromMap.
func stringSliceFromMap(data map[string]interface{}, key string) []string {
	val, ok := data[key]
	if !ok {
		return nil
	}

	items, isList := val.([]interface{})
	if !isList {
		return nil
	}

	result := make([]string, 0, len(items))

	for _, item := range items {
		if s, isStr := item.(string); isStr {
			result = append(result, s)
		}
	}

	return result
}

// idSanitizerUnsafe matches every run of characters the production
// sanitizer replaces in artifact path segments.
var idSanitizerUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// sanitizeID is a port of the production sanitizeID: collapse every
// unsafe run to a single "-". Not injective and never trims a trailing
// hyphen — build-code subjects end with "-".
func sanitizeID(id string) string {
	return idSanitizerUnsafe.ReplaceAllString(id, "-")
}
