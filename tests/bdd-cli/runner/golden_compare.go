package runner

import (
	"errors"
	"fmt"
	"strings"
)

// ErrGoldenUnreadable is returned when a recorded outcome exists but
// cannot be used — a schema this build does not know how to read.
var ErrGoldenUnreadable = errors.New("golden tree unreadable")

// goldenDiffLines caps how much of a differing file a failure prints.
// Enough to see what moved; not so much that one changed story buries
// the other failures.
const goldenDiffLines = 12

// CompareGolden checks a run's diff against the recorded outcome and
// returns one failure line per discrepancy.
//
// Strict in both directions: a file the recording has and the run did
// not produce is a failure, and so is one the run produced that the
// recording does not have. A regression that only ADDS output is still a
// regression, and the whole point of replaying is that nothing about the
// engine's output is supposed to have moved.
func CompareGolden(golden *GoldenTree, diff []FileChange) []string {
	expected := make(map[string]GoldenEntry, len(golden.Files))
	for _, entry := range golden.Files {
		expected[entry.Path] = entry
	}

	actual := make(map[string]GoldenEntry, len(diff))

	var order []string

	for _, change := range diff {
		if isScratchPath(change.Path) {
			continue
		}

		actual[change.Path] = newGoldenEntry(change)
		order = append(order, change.Path)
	}

	var failures []string

	// Recorded order first, so a report reads in the same order as the
	// golden file itself.
	for _, entry := range golden.Files {
		got, present := actual[entry.Path]
		if !present {
			failures = append(failures, fmt.Sprintf(
				"golden: %s (%s in the recording) was not produced by this run", entry.Path, entry.Kind))

			continue
		}

		failures = append(failures, compareEntry(entry, got)...)
	}

	for _, path := range order {
		if _, recorded := expected[path]; !recorded {
			failures = append(failures, fmt.Sprintf(
				"golden: %s was produced by this run but is not in the recording (%d bytes)",
				path, actual[path].Size))
		}
	}

	return failures
}

func compareEntry(want, got GoldenEntry) []string {
	if want.Kind != got.Kind {
		return []string{fmt.Sprintf("golden: %s was %s in the recording, %s in this run",
			want.Path, want.Kind, got.Kind)}
	}

	if want.SHA256 == got.SHA256 {
		return nil
	}

	line := fmt.Sprintf("golden: %s differs from the recording (%d bytes recorded, %d now)",
		want.Path, want.Size, got.Size)

	// The digests decide; the text only explains. Absent for binary or
	// oversized content, where the sizes above are the whole story.
	if want.Text != "" && got.Text != "" {
		line += "\n" + indentBlock(firstDifference(want.Text, got.Text))
	}

	return []string{line}
}

// firstDifference renders the neighbourhood of the first line that
// moved. A whole-file diff would be noise: by the time two files of the
// same run disagree, the first divergence is the story.
func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	start := divergedAt(wantLines, gotLines)

	var buf strings.Builder

	fmt.Fprintf(&buf, "first difference at line %d:\n", start+1)

	for offset := range goldenDiffLines {
		line, more := renderLine(wantLines, gotLines, start+offset)
		if !more {
			break
		}

		buf.WriteString(line)
	}

	return strings.TrimRight(buf.String(), "\n")
}

// divergedAt is the index of the first line the two sides disagree on.
func divergedAt(wantLines, gotLines []string) int {
	index := 0
	for index < len(wantLines) && index < len(gotLines) && wantLines[index] == gotLines[index] {
		index++
	}

	return index
}

// renderLine formats one line of the difference, reporting false once
// both sides are exhausted.
//
// Lines that agree print once, unmarked: rendering them as a ± pair
// makes an identical tail look like a wall of differences and buries the
// one line that moved.
func renderLine(wantLines, gotLines []string, index int) (string, bool) {
	want, hasWant := lineAt(wantLines, index)
	got, hasGot := lineAt(gotLines, index)

	switch {
	case !hasWant && !hasGot:
		return "", false
	case hasWant && hasGot && want == got:
		return fmt.Sprintf("    %s\n", want), true
	case !hasGot:
		return fmt.Sprintf("  - %s\n", want), true
	case !hasWant:
		return fmt.Sprintf("  + %s\n", got), true
	default:
		return fmt.Sprintf("  - %s\n  + %s\n", want, got), true
	}
}

func lineAt(lines []string, index int) (string, bool) {
	if index < 0 || index >= len(lines) {
		return "", false
	}

	return lines[index], true
}

func indentBlock(block string) string {
	lines := strings.Split(block, "\n")
	for index, line := range lines {
		lines[index] = "    " + line
	}

	return strings.Join(lines, "\n")
}
