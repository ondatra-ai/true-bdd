package main

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// VerdictClass classifies one evaluator response with production-parity
// parsing. Only the two canonical classes may earn branch credit.
type VerdictClass string

// Verdict classes. Canonical pass/fail credit branches; everything else
// is a diagnostic (protocol accident or non-canonical answer), matching
// the strict rule from the accepted measurement design.
const (
	ClassPassCanonical    VerdictClass = "pass"
	ClassFailCanonical    VerdictClass = "fail"
	ClassNonCanonical     VerdictClass = "non_canonical"
	ClassProtocolNoMarker VerdictClass = "protocol_no_marker"
	ClassProtocolBadYAML  VerdictClass = "protocol_bad_yaml"
)

// Credits reports whether the class is one of the two canonical
// verdicts that may earn coverage credit.
func (c VerdictClass) Credits() bool {
	return c == ClassPassCanonical || c == ClassFailCanonical
}

// evaluatorResult mirrors the production resultYAML exactly: a flexible
// answer node plus typed context and fix_prompt — a wrongly typed
// auxiliary field is a production parse failure and must classify as
// protocol_bad_yaml here too.
type evaluatorResult struct {
	Answer    yaml.Node `yaml:"answer"`
	Context   []string  `yaml:"context,omitempty"`
	FixPrompt string    `yaml:"fix_prompt,omitempty"`
}

// ClassifyResponse re-parses a saved evaluator response with the
// production extraction rules (exact then fuzzy FILE markers, then
// YAML with a flexible answer node).
func ClassifyResponse(response, expectedPath string) VerdictClass {
	content := extractFileContent(response, expectedPath)
	if content == "" {
		return ClassProtocolNoMarker
	}

	var result evaluatorResult

	err := yaml.Unmarshal([]byte(stripMarkdownFences(content)), &result)
	if err != nil {
		return ClassProtocolBadYAML
	}

	return classifyAnswerNode(result.Answer)
}

// classifyAnswerNode maps the parsed answer node to a verdict class:
// canonical pass/fail only for scalar answers whose trimmed,
// case-folded value is exactly "pass" or "fail".
func classifyAnswerNode(node yaml.Node) VerdictClass {
	if node.IsZero() || node.Kind != yaml.ScalarNode {
		return ClassNonCanonical
	}

	switch strings.ToLower(strings.TrimSpace(node.Value)) {
	case "pass":
		return ClassPassCanonical
	case "fail":
		return ClassFailCanonical
	default:
		return ClassNonCanonical
	}
}

// stripMarkdownFences is a port of the production stripMarkdownFences:
// some models wrap the YAML payload in ```/```yaml fences inside the
// FILE block; production strips them before parsing.
func stripMarkdownFences(content string) string {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = content[idx+1:]
		} else {
			content = strings.TrimPrefix(content, "```")
		}
	}

	content = strings.TrimSpace(content)
	content = strings.TrimSuffix(content, "```")

	return strings.TrimSpace(content)
}

// extractFileContent is a port of the production ExtractFileContent:
// exact-match FILE markers first, then the fuzzy regex fallback.
func extractFileContent(response, path string) string {
	startMarker := "=== FILE_START: " + path + " ==="
	endMarker := "=== FILE_END: " + path + " ==="

	startIdx := strings.Index(response, startMarker)
	if startIdx != -1 {
		contentStart := startIdx + len(startMarker)

		endIdx := strings.Index(response[contentStart:], endMarker)
		if endIdx != -1 {
			return strings.TrimSpace(response[contentStart : contentStart+endIdx])
		}
	}

	return extractFuzzy(response, path)
}

// extractFuzzy handles AI formatting variations around the markers,
// mirroring the production fallback regexes.
func extractFuzzy(response, path string) string {
	escapedPath := regexp.QuoteMeta(path)

	startRe, err := regexp.Compile(`={2,4}\s*"?\s*FILE_START:\s*` + escapedPath + `\s*"?\s*={2,4}`)
	if err != nil {
		return ""
	}

	endRe, err := regexp.Compile(`={2,4}\s*"?\s*FILE_END:\s*` + escapedPath + `\s*"?\s*={2,4}`)
	if err != nil {
		return ""
	}

	startLoc := startRe.FindStringIndex(response)
	if startLoc == nil {
		return ""
	}

	remaining := response[startLoc[1]:]

	endLoc := endRe.FindStringIndex(remaining)
	if endLoc == nil {
		return ""
	}

	return strings.TrimSpace(remaining[:endLoc[0]])
}
