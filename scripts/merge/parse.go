package merge

import (
	"regexp"
	"strings"
)

// CodeRabbit puts findings in two places and only one is a thread: actionable
// comments become review threads, "Nitpick/Outside diff/Duplicate" comments
// live only in the review body. Threads alone saw 16 of 28 findings on PR #70.

const (
	openTag  = "<details>"
	closeTag = "</details>"
)

//nolint:gochecknoglobals // the machinery's vocabulary; constants in all but syntax.
var (
	noiseSummaries = []string{
		"Analysis chain", "Prompt for AI Agents", "Skipped due to learnings",
		"Learnings used", "Review details", "Review info", "Run configuration",
		"Files selected for processing", "Files with no reviewable changes",
		"Files skipped from review", "Commits", "Autofix", "Tip",
	}
	bodyOnlySections = []string{
		"Nitpick comments", "Outside diff range comments", "Duplicate comments",
	}

	sectionRE     = regexp.MustCompile(`(` + strings.Join(quoted(bodyOnlySections), "|") + `)\s*\((\d+)\)`)
	fileHeadingRE = regexp.MustCompile(`^(\S.*?)\s*\((\d+)\)$`)
	// Findings after the first in a file block are preceded by a markdown rule.
	lineRangeRE   = regexp.MustCompile("^(?:\\s*-{3,}\\s*)*\\s*`([\\d\\-]+)`\\s*:")
	actionableRE  = regexp.MustCompile(`Actionable comments posted:\s*(\d+)`)
	markerRE      = regexp.MustCompile(`<!--\s*cr-comment:v\d+:[0-9a-f]+\s*-->`)
	summaryRE     = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	tagRE         = regexp.MustCompile(`<[^>]+>`)
	blockquoteRE  = regexp.MustCompile(`</?blockquote>`)
	htmlCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)
	// "_🟠 Major_" — the emoji sits INSIDE the underscores, and `_` is a word
	// character so a trailing \b matches nothing.
	severityRE = regexp.MustCompile(`(?i)_[^_]*(Critical|Major|Minor|Trivial)[^_]*_`)
	titleRE    = regexp.MustCompile(`(?m)^\s*>?\s*\*\*(.+?)\*\*\s*$`)
	bodyLineRE = regexp.MustCompile("(?m)^\\s*>?\\s*`(\\d+(?:-\\d+)?)`\\s*:")
	blankRunRE = regexp.MustCompile(`\n{3,}`)
)

// block is the span of one <details> element.
type block struct {
	contentStart int
	contentEnd   int
	end          int
}

// findBlock is the content span of the <details> opening at start,
// nesting-aware. CodeRabbit nests <details> three deep.
func findBlock(text string, start int) (block, bool) {
	depth, index := 0, start

	for index < len(text) {
		nextOpen := indexFrom(text, openTag, index)
		nextClose := indexFrom(text, closeTag, index)

		if nextClose == -1 {
			return block{}, false
		}

		if nextOpen != -1 && nextOpen < nextClose {
			depth++
			index = nextOpen + len(openTag)

			continue
		}

		depth--
		if depth == 0 {
			return block{
				contentStart: start + len(openTag),
				contentEnd:   nextClose,
				end:          nextClose + len(closeTag),
			}, true
		}

		index = nextClose + len(closeTag)
	}

	return block{}, false
}

// summaryOf is a block's <summary>, stripped of markup.
func summaryOf(inner string) string {
	match := summaryRE.FindStringSubmatch(inner)
	if match == nil {
		return ""
	}

	return strings.TrimSpace(tagRE.ReplaceAllString(match[1], ""))
}

func isNoise(summary string) bool {
	for _, noise := range noiseSummaries {
		if strings.Contains(summary, noise) {
			return true
		}
	}

	return false
}

// clean drops the machinery blocks, keeps everything else headed by its
// summary. Denylist, not allowlist: proposed-fix blocks use 4+ different
// summaries, so an allowlist would silently drop the one that matters.
func clean(body string) string {
	return cleanAt(body, 0)
}

// cleanAt tidies at EVERY level, not once at the top — load-bearing for
// marker placement and indent: the recursive result is trimmed before
// splicing in.
func cleanAt(body string, depth int) string {
	var out strings.Builder

	index := 0

	for {
		start := indexFrom(body, openTag, index)
		if start == -1 {
			out.WriteString(body[index:])

			break
		}

		span, ok := findBlock(body, start)
		if !ok {
			out.WriteString(body[index:])

			break
		}

		inner := body[span.contentStart:span.contentEnd]
		out.WriteString(body[index:start])

		if summary := summaryOf(inner); !isNoise(summary) {
			stripped := replaceFirst(summaryRE, inner)

			if summary != "" {
				out.WriteString("\n")
				out.WriteString(strings.Repeat("  ", depth+1))
				out.WriteString("▸ ")
				out.WriteString(summary)
				out.WriteString("\n")
			}

			out.WriteString(cleanAt(stripped, depth+1))
		}

		index = span.end
	}

	text := htmlCommentRE.ReplaceAllString(out.String(), "")
	text = blockquoteRE.ReplaceAllString(text, "")

	return strings.TrimSpace(blankRunRE.ReplaceAllString(text, "\n\n"))
}

// bodyFinding is one finding lifted out of a review body.
type bodyFinding struct {
	section string
	path    string
	lines   string
	text    string
}

// extractBodyFindings reads the sections GitHub models as nothing at all.
func extractBodyFindings(review ghReview) []bodyFinding {
	var findings []bodyFinding

	for _, match := range sectionRE.FindAllStringSubmatchIndex(review.Body, -1) {
		blockStart := strings.LastIndex(review.Body[:match[0]], openTag)
		if blockStart == -1 {
			continue
		}

		span, ok := findBlock(review.Body, blockStart)
		if !ok {
			continue
		}

		section := review.Body[match[2]:match[3]]
		findings = append(findings,
			findingsInSection(review.Body[span.contentStart:span.contentEnd], section)...)
	}

	return findings
}

func findingsInSection(sectionBody, section string) []bodyFinding {
	var findings []bodyFinding

	index := 0

	for {
		start := indexFrom(sectionBody, openTag, index)
		if start == -1 {
			break
		}

		span, ok := findBlock(sectionBody, start)
		if !ok {
			break
		}

		inner := sectionBody[span.contentStart:span.contentEnd]
		if heading := fileHeadingRE.FindStringSubmatch(summaryOf(inner)); heading != nil {
			findings = append(findings, chunksOf(replaceFirst(summaryRE, inner), section, heading[1])...)
		}

		index = span.end
	}

	return findings
}

// chunksOf splits a file block into its findings. A finding ends at its
// cr-comment marker; what trails the LAST marker is the block's footer —
// counting it would inflate the total per file.
func chunksOf(stripped, section, path string) []bodyFinding {
	chunks := markerRE.Split(stripped, -1)
	if len(chunks) > 1 {
		chunks = chunks[:len(chunks)-1]
	}

	findings := make([]bodyFinding, 0, len(chunks))

	for _, chunk := range chunks {
		text := clean(chunk)
		if text == "" {
			continue
		}

		lines := "?"
		if match := lineRangeRE.FindStringSubmatch(
			strings.TrimSpace(blockquoteRE.ReplaceAllString(chunk, ""))); match != nil {
			lines = match[1]
		}

		findings = append(findings, bodyFinding{section: section, path: path, lines: lines, text: text})
	}

	return findings
}

// --------------------------------------------------------------- one finding

func severityOf(body string) string {
	if match := severityRE.FindStringSubmatch(body); match != nil {
		return strings.ToLower(match[1])
	}

	return "?"
}

const titleLimit = 200

func titleOf(body string) string {
	if match := titleRE.FindStringSubmatch(body); match != nil {
		return truncate(strings.TrimSpace(match[1]), titleLimit)
	}

	for line := range strings.SplitSeq(body, "\n") {
		stripped := strings.TrimSpace(strings.TrimLeft(line, "> "))
		if stripped == "" || strings.HasPrefix(stripped, "_") || strings.HasPrefix(stripped, "`") ||
			strings.HasPrefix(stripped, "|") || strings.HasPrefix(stripped, "▸") ||
			strings.HasPrefix(stripped, "-") || strings.HasPrefix(stripped, "#") {
			continue
		}

		return truncate(strings.TrimSpace(strings.Trim(stripped, "*")), titleLimit)
	}

	return truncate(strings.TrimSpace(body), titleLimit)
}

func lineFromBody(body string) string {
	if match := bodyLineRE.FindStringSubmatch(body); match != nil {
		return match[1]
	}

	return "?"
}

func isBot(login string) bool {
	return strings.Contains(strings.ToLower(login), "coderabbit")
}

// ------------------------------------------------------------------ helpers

// replaceFirst removes the first match only, which regexp cannot express.
func replaceFirst(pattern *regexp.Regexp, text string) string {
	span := pattern.FindStringIndex(text)
	if span == nil {
		return text
	}

	return text[:span[0]] + text[span[1]:]
}

// indexFrom is strings.Index from an offset, in the original coordinates.
func indexFrom(text, needle string, from int) int {
	if from >= len(text) {
		return -1
	}

	found := strings.Index(text[from:], needle)
	if found == -1 {
		return -1
	}

	return from + found
}

func quoted(items []string) []string {
	out := make([]string, len(items))
	for index, item := range items {
		out[index] = regexp.QuoteMeta(item)
	}

	return out
}
