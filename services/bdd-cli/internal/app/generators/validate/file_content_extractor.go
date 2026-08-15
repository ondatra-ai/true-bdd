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
	// with a bare `=== FILE_END ===`. The opening marker already pinned
	// which file this block is, so requiring the path twice only threw
	// away a block that was otherwise complete and correct.
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

// fencedBlock matches content wrapped ENTIRELY in one markdown code
// fence: backticks or tildes, three or more, with an optional language
// tag, closed by the same character. CommonMark allows all of those, and
// a model that opens with ````yaml is doing nothing unusual — matching
// only three backticks would leave that fence in the payload and put the
// YAML parse back where it started.
var fencedBlock = regexp.MustCompile(
	"(?s)\\A(?:```+|~~~+)[a-zA-Z0-9_+-]*[ \\t]*\\r?\\n(.*?)\\r?\\n?(?:```+|~~~+)[ \\t]*\\z")

// stripCodeFence removes a markdown fence wrapping the whole block.
//
// The markers already say what the block is and where it goes, so a
// fence inside them carries no information — but the engine writes the
// content to a file and parses it, and a stray ``` is a YAML syntax
// error that takes the entire command down with "found character that
// cannot start any token". A coder model emitting fenced output is
// normal, occasional, and not worth failing a run over; this is the same
// tolerance the marker matching above already extends.
//
// Only a fence around the WHOLE block is removed. A fence in the middle
// belongs to the content — a fix prompt legitimately contains example
// YAML — and removing those would corrupt it.
func stripCodeFence(content string) string {
	match := fencedBlock.FindStringSubmatch(content)
	if match == nil {
		return content
	}

	return strings.TrimSpace(match[1])
}
