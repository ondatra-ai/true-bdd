package validate

import (
	"regexp"
	"strings"
)

// ExtractFileContent extracts content between FILE_START and FILE_END markers.
// It first attempts exact match, then falls back to a regex-based fuzzy match
// to handle common AI formatting variations (e.g., extra quotes, missing spaces).
func ExtractFileContent(response, path string) string {
	// Try exact match first (fast path).
	startMarker := "=== FILE_START: " + path + " ==="
	endMarker := "=== FILE_END: " + path + " ==="

	startIdx := strings.Index(response, startMarker)
	if startIdx != -1 {
		contentStart := startIdx + len(startMarker)
		endIdx := strings.Index(response[contentStart:], endMarker)

		if endIdx != -1 {
			return stripCodeFence(strings.TrimSpace(response[contentStart : contentStart+endIdx]))
		}
	}

	// Fuzzy match: handle AI formatting variations like extra quotes,
	// missing/extra spaces around ===, etc.
	// Escape the path for regex (it contains dots, slashes, etc.)
	escapedPath := regexp.QuoteMeta(path)
	pattern := `={2,4}\s*"?\s*FILE_START:\s*` + escapedPath + `\s*"?\s*={2,4}`
	startRe := regexp.MustCompile(pattern)

	// The path on the closing marker is optional: crush routinely closes
	// with a bare `=== FILE_END ===`, and requiring the path twice used to
	// throw away an otherwise complete and correct block.
	endPattern := `={2,4}\s*"?\s*FILE_END:?\s*(?:` + escapedPath + `)?\s*"?\s*={2,4}`
	endRe := regexp.MustCompile(endPattern)

	startLoc := startRe.FindStringIndex(response)
	if startLoc == nil {
		return ""
	}

	contentStart := startLoc[1]
	remaining := response[contentStart:]

	endLoc := endRe.FindStringIndex(remaining)
	if endLoc == nil {
		return ""
	}

	return stripCodeFence(strings.TrimSpace(remaining[:endLoc[0]]))
}

// fencedBlock matches content wrapped ENTIRELY in one markdown fence:
// backticks or tildes, three OR MORE (CommonMark-legal), with an optional
// language tag — fixing the count at exactly three would leave a ````yaml fence in place.
var fencedBlock = regexp.MustCompile(
	"(?s)\\A(?:```+|~~~+)[a-zA-Z0-9_+-]*[ \\t]*\\r?\\n(.*?)\\r?\\n?(?:```+|~~~+)[ \\t]*\\z")

// stripCodeFence removes a markdown fence wrapping the whole block. A stray
// ``` left in unparsed content is a YAML syntax error that takes the whole
// command down; only a WHOLE-block fence is stripped — one in the middle belongs to the content.
func stripCodeFence(content string) string {
	match := fencedBlock.FindStringSubmatch(content)
	if match == nil {
		return content
	}

	return strings.TrimSpace(match[1])
}
