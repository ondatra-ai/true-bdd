package merge

// The comment machinery is unexported; the parity test reaches it through
// this export_test.go seam, which the compiler drops from any non-test
// build.

import "time"

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

// GateRun is a folded CI gates job, for the gate-time test.
type GateRun struct {
	Total time.Duration
	Attrs []any
}

// GateTimings folds a jobs payload exactly as reportGateTime does.
func GateTimings(payload []byte) (GateRun, bool) {
	folded, ok := gateTimings(payload)
	if !ok {
		return GateRun{}, false
	}

	return GateRun{Total: folded.total, Attrs: folded.attrs()}, true
}

// PanicStop unwinds the way dief and usage do, so a test can prove guard
// converts a stop and re-panics anything else.
func PanicStop(message string) { panic(stopSentinel{message: message}) }

// Guard is the recover-to-error wrapper Execute and Embed are built on.
func Guard(body func()) error { return guard(body) }

