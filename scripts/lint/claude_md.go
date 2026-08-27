package lint

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"znkr.io/diff/textdiff"
)

const (
	claudeMD    = "CLAUDE.md"
	maxCols     = 80
	beginMarker = "<!-- KARPATHY:BEGIN"
	endMarker   = "<!-- KARPATHY:END"
	fetchBudget = 30 * time.Second

	upstreamURL = "https://raw.githubusercontent.com/" +
		"multica-ai/andrej-karpathy-skills/main/CLAUDE.md"
)

// ClaudeMD lints CLAUDE.md: no line over maxCols outside the mirrored block,
// and the block itself byte-identical to upstream. Cheapest check first.
func ClaudeMD(out io.Writer) error {
	raw, err := os.ReadFile(claudeMD)
	if err != nil {
		return fmt.Errorf("reading %s: %w", claudeMD, err)
	}

	lines := splitLines(string(raw))

	begin, end, err := markerBounds(lines)
	if err != nil {
		return err
	}

	err = checkWidth(out, lines, begin, end)
	if err != nil {
		return err
	}

	err = checkMirror(out, lines[begin:end-1])
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "lint-claude.md: OK (all lines ≤%d cols; mirror at %d-%d matches upstream)\n",
		maxCols, begin, end)

	return nil
}

// markerBounds returns the two markers' 1-indexed line numbers.
func markerBounds(lines []string) (int, int, error) {
	begin, end, begins, ends := 0, 0, 0, 0

	for index, line := range lines {
		switch {
		case strings.HasPrefix(line, beginMarker):
			begin, begins = index+1, begins+1
		case strings.HasPrefix(line, endMarker):
			end, ends = index+1, ends+1
		}
	}

	if begins != 1 || ends != 1 {
		return 0, 0, fmt.Errorf(
			"%w: expected exactly one BEGIN and one END marker in %s, found %d and %d",
			ErrFailed, claudeMD, begins, ends)
	}

	if begin >= end {
		return 0, 0, fmt.Errorf("%w: END marker (line %d) precedes BEGIN (line %d)",
			ErrFailed, end, begin)
	}

	return begin, end, nil
}

// checkWidth skips strictly BETWEEN the markers, so the marker lines
// themselves are held to the rule — they are ours to write.
func checkWidth(out io.Writer, lines []string, begin, end int) error {
	var wide []string

	for index, line := range lines {
		number := index + 1
		if number > begin && number < end {
			continue
		}

		if width := utf8.RuneCountInString(line); width > maxCols {
			wide = append(wide, fmt.Sprintf("    line %d: %d chars", number, width))
		}
	}

	if len(wide) == 0 {
		return nil
	}

	_, _ = fmt.Fprintf(out, `lint-claude.md: lines over %d columns:
%s
Reflow them. Prose wraps; a long URL or table row that cannot wrap belongs
behind a shorter reference.
`, maxCols, strings.Join(wide, "\n"))

	return ErrFailed
}

// checkMirror compares the block to the live upstream bytes. No network, no
// verdict: a failed fetch is a hard failure, never a pass — "could not check"
// must not read as "checked and fine".
func checkMirror(out io.Writer, block []string) error {
	upstream, err := fetchUpstream()
	if err != nil {
		return err
	}

	ours := strings.Join(block, "\n") + "\n"
	if ours == upstream {
		return nil
	}

	_, _ = fmt.Fprintf(out, `lint-claude.md: the KARPATHY block in %s has drifted
from upstream ('-' is upstream, '+' is %s):
%s
Paste the upstream bytes back between the markers — see the ClaudeMD doc.
`, claudeMD, claudeMD, textdiff.Unified(upstream, ours))

	return ErrFailed
}

func fetchUpstream() (string, error) {
	client := &http.Client{Timeout: fetchBudget}

	response, err := client.Get(upstreamURL) //nolint:noctx // the client's timeout is the deadline.
	if err != nil {
		return "", fmt.Errorf("%w: could not fetch %s: %w — this gate needs the network, "+
			"and an unreachable upstream is a failure, not a pass",
			ErrFailed, upstreamURL, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %s returned %s", ErrFailed, upstreamURL, response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("%w: reading %s: %w", ErrFailed, upstreamURL, err)
	}

	if len(body) == 0 {
		return "", fmt.Errorf("%w: %s returned an empty body", ErrFailed, upstreamURL)
	}

	return string(body), nil
}
