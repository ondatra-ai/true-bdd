package merge

import "time"

// The comment machinery is unexported; the parity test reaches it through
// this export_test.go seam, which the compiler drops from any non-test
// build.

func botReview(body string) ghReview {
	return ghReview{ID: 1, Body: body, User: ghUser{Login: "coderabbitai[bot]"}}
}

// ParsedFinding mirrors the golden's rows.
type ParsedFinding struct {
	Section      string `json:"section"`
	Path         string `json:"path"`
	Lines        string `json:"lines"`
	Text         string `json:"text"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	LineFromBody string `json:"line_from_body"`
}

// ParseReviewBody extracts the body-only findings and annotates each one.
func ParseReviewBody(body string) []ParsedFinding {
	extracted := extractBodyFindings(botReview(body))

	parsed := make([]ParsedFinding, 0, len(extracted))
	for _, finding := range extracted {
		parsed = append(parsed, ParsedFinding{
			Section:      finding.section,
			Path:         finding.path,
			Lines:        finding.lines,
			Text:         finding.text,
			Severity:     severityOf(finding.text),
			Title:        titleOf(finding.text),
			LineFromBody: lineFromBody(finding.text),
		})
	}

	return parsed
}

// Clean drops the machinery blocks from a comment body.
func Clean(body string) string { return clean(body) }

// SeverityOf reads the reviewer's severity label.
func SeverityOf(body string) string { return severityOf(body) }

// TitleOf reads a finding's claim.
func TitleOf(body string) string { return titleOf(body) }

// ClaimedCounts is what a review body says about itself.
func ClaimedCounts(body string) map[string]int {
	return claimedCounts([]ghReview{botReview(body)})
}

// WorthAPostmortem is the gate the merge loop's tail sits behind.
func WorthAPostmortem(automatic bool, total time.Duration) (bool, string) {
	return worthAPostmortem(automatic, total)
}

// PostmortemFloor is how long a clean automatic run must take to earn one.
const PostmortemFloor = postmortemFloor

// PostmortemPrompt is the embedded prompt, before anything is substituted.
func PostmortemPrompt() string { return postmortemPrompt }

// RenderPostmortemPrompt substitutes the transcript and the timing table.
func RenderPostmortemPrompt(transcript, timings string) string {
	return renderPostmortemPrompt(transcript, timings)
}

// ReadLedger sums a pr-commit timing ledger.
func ReadLedger(path string) time.Duration { return readLedger(path) }
