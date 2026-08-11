package reportserver

import (
	"strconv"
	"strings"

	"znkr.io/diff"
	"znkr.io/diff/textdiff"
)

// Caps on one text diff's payload.
//
// The pathological case is two entirely unrelated texts, where hunks
// cover 100% of both sides and a ~70KB prompt pair becomes a ~140KB
// response. Truncation is reported rather than silent: a diff that
// quietly stops half way reads as "the rest is identical", which is the
// one thing it must never imply.
const (
	maxDiffEdits       = 4000
	maxDiffBytes       = 512 * 1024
	defaultDiffContext = 3
)

// TextDiff is two artifacts aligned line by line.
type TextDiff struct {
	Identical  bool       `json:"identical"`
	Truncated  bool       `json:"truncated"`
	LeftLines  int        `json:"left_lines"`
	RightLines int        `json:"right_lines"`
	LeftBytes  int        `json:"left_bytes"`
	RightBytes int        `json:"right_bytes"`
	Hunks      []TextHunk `json:"hunks"`
}

// TextHunk is one contiguous region of change plus its context.
type TextHunk struct {
	LeftStart  int        `json:"left_start"`
	RightStart int        `json:"right_start"`
	Lines      []TextLine `json:"lines"`
}

// TextLine is one line of a hunk.
type TextLine struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

// DiffText aligns two texts by line, bounded by the caps above.
func DiffText(left, right string, context int) TextDiff {
	if context < 0 {
		context = defaultDiffContext
	}

	result := TextDiff{
		Identical:  left == right,
		LeftLines:  countLines(left),
		RightLines: countLines(right),
		LeftBytes:  len(left),
		RightBytes: len(right),
		Hunks:      []TextHunk{},
	}

	if result.Identical {
		return result
	}

	edits := 0
	bytes := 0

	// IndentHeuristic shifts edit boundaries onto indentation, which is
	// what makes a YAML or prompt diff read as "this block changed"
	// rather than as an offset smear across two blocks.
	hunks := textdiff.Hunks(left, right, diff.Context(context), textdiff.IndentHeuristic())

	for _, hunk := range hunks {
		if edits >= maxDiffEdits || bytes >= maxDiffBytes {
			result.Truncated = true

			break
		}

		out := TextHunk{
			// Line numbers are zero-based on the wire's source; readers
			// count from 1.
			LeftStart:  hunk.LineNoX + 1,
			RightStart: hunk.LineNoY + 1,
			Lines:      make([]TextLine, 0, len(hunk.Edits)),
		}

		for _, edit := range hunk.Edits {
			if edits >= maxDiffEdits || bytes >= maxDiffBytes {
				result.Truncated = true

				break
			}

			line := strings.TrimSuffix(edit.Line, "\n")
			out.Lines = append(out.Lines, TextLine{Op: textOp(edit.Op), Text: line})

			edits++
			bytes += len(line)
		}

		result.Hunks = append(result.Hunks, out)
	}

	return result
}

// textOp names a line-level operation for the wire.
func textOp(op diff.Op) string {
	switch op {
	case diff.Delete:
		return opDelete
	case diff.Insert:
		return opInsert
	case diff.Match:
		return opMatch
	default:
		return opMatch
	}
}

// countLines counts a text's lines, treating empty as zero rather than
// one so an absent artifact does not read as a one-line file.
func countLines(text string) int {
	if text == "" {
		return 0
	}

	return strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
}

// splitLines splits a body into lines for list diffing.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}

	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// itoa is strconv.Itoa, for the many inline scalar renders.
func itoa(value int) string {
	return strconv.Itoa(value)
}
