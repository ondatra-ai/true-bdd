package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"bdd-cli/src/internal/domain/models/checklist"
)

const (
	maxQuestionLen        = 80
	maxQuestionLenFixList = 80 // Longer question display for fix prompts list
	separatorLine         = "================================================================================"
	percentMultiplier     = 100
	minTruncateLen        = 3
)

// TableRenderer renders checklist reports as ASCII tables.
type TableRenderer struct {
	writer io.Writer
}

// NewTableRenderer creates a new table renderer writing to stdout.
func NewTableRenderer() *TableRenderer {
	return &TableRenderer{
		writer: os.Stdout,
	}
}

// RenderReport renders a checklist report as an ASCII table.
// If showFixPrompts is true, fix prompts for failed checks are displayed.
func (r *TableRenderer) RenderReport(report *checklist.ChecklistReport, showFixPrompts bool) {
	r.renderHeader(report)
	r.renderTable(report)
	r.renderSummary(report)

	if showFixPrompts {
		r.renderFixPrompts(report)
	}
}

// renderHeader renders the report header.
func (r *TableRenderer) renderHeader(report *checklist.ChecklistReport) {
	_, _ = fmt.Fprintln(r.writer, separatorLine)
	_, _ = fmt.Fprintf(r.writer, "CHECKLIST VALIDATION - %s: %s\n",
		report.SubjectID, report.SubjectTitle)
	_, _ = fmt.Fprintln(r.writer, separatorLine)
	_, _ = fmt.Fprintln(r.writer)
}

// renderTable renders the results table.
func (r *TableRenderer) renderTable(report *checklist.ChecklistReport) {
	const (
		columnPadding = 2
		answerMaxLen  = 12
	)

	tabWriter := tabwriter.NewWriter(r.writer, 0, 0, columnPadding, ' ', 0)

	// Header
	_, _ = fmt.Fprintln(tabWriter, "SECTION\tQUESTION\tACTUAL\tSTATUS")
	_, _ = fmt.Fprintln(tabWriter, "-------\t--------\t------\t------")

	// Results
	for _, result := range report.Results {
		question := truncateString(result.Question, maxQuestionLen)
		actual := summarizeAnswer(result.ActualAnswer, answerMaxLen)
		status := r.formatStatus(result.Status)

		_, _ = fmt.Fprintf(tabWriter, "%s\t%s\t%s\t%s\n",
			result.SectionPath,
			question,
			actual,
			status,
		)
	}

	_ = tabWriter.Flush()
	_, _ = fmt.Fprintln(r.writer)
}

// renderSummary renders the report summary.
func (r *TableRenderer) renderSummary(report *checklist.ChecklistReport) {
	_, _ = fmt.Fprintln(r.writer, separatorLine)
	_, _ = fmt.Fprintln(r.writer, "SUMMARY")
	_, _ = fmt.Fprintln(r.writer, separatorLine)
	_, _ = fmt.Fprintf(r.writer, "Total Prompts: %d\n", report.Summary.TotalPrompts)
	_, _ = fmt.Fprintf(r.writer, "PASS: %d (%.1f%%)\n",
		report.Summary.PassCount,
		r.calculatePercentage(report.Summary.PassCount, report.Summary.TotalPrompts))
	_, _ = fmt.Fprintf(r.writer, "WARN: %d (%.1f%%)\n",
		report.Summary.WarnCount,
		r.calculatePercentage(report.Summary.WarnCount, report.Summary.TotalPrompts))
	_, _ = fmt.Fprintf(r.writer, "FAIL: %d (%.1f%%)\n",
		report.Summary.FailCount,
		r.calculatePercentage(report.Summary.FailCount, report.Summary.TotalPrompts))
	_, _ = fmt.Fprintf(r.writer, "SKIP: %d\n", report.Summary.SkipCount)
	_, _ = fmt.Fprintln(r.writer)
	_, _ = fmt.Fprintf(r.writer, "Overall: %s\n", report.GetOverallStatus())
	_, _ = fmt.Fprintln(r.writer, separatorLine)
}

// formatStatus formats the status with visual indicators.
func (r *TableRenderer) formatStatus(status checklist.Status) string {
	switch status {
	case checklist.StatusPass:
		return "PASS"
	case checklist.StatusWarn:
		return "WARN"
	case checklist.StatusFail:
		return "FAIL"
	case checklist.StatusSkip:
		return "SKIP"
	default:
		return string(status)
	}
}

// calculatePercentage calculates percentage safely.
func (r *TableRenderer) calculatePercentage(count, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(count) / float64(total) * percentMultiplier
}

// renderFixPrompts renders fix prompts for failed validations.
func (r *TableRenderer) renderFixPrompts(report *checklist.ChecklistReport) {
	// Collect results with fix prompts
	var fixes []checklist.ValidationResult

	for _, result := range report.Results {
		if result.Status == checklist.StatusFail && result.FixPrompt != "" {
			fixes = append(fixes, result)
		}
	}

	if len(fixes) == 0 {
		return
	}

	_, _ = fmt.Fprintln(r.writer)
	_, _ = fmt.Fprintln(r.writer, separatorLine)
	_, _ = fmt.Fprintln(r.writer, "FIX PROMPTS")
	_, _ = fmt.Fprintln(r.writer, separatorLine)

	for i, fix := range fixes {
		_, _ = fmt.Fprintf(r.writer, "\n### Fix %d: %s\n", i+1, fix.SectionPath)
		_, _ = fmt.Fprintf(r.writer, "Question: %s\n", truncateString(fix.Question, maxQuestionLenFixList))
		_, _ = fmt.Fprintln(r.writer)
		_, _ = fmt.Fprintln(r.writer, fix.FixPrompt)
	}

	_, _ = fmt.Fprintln(r.writer, separatorLine)
}

// summarizeAnswer renders the ACTUAL column for a row by truncating the
// answer to maxLen characters.
func summarizeAnswer(actual string, maxLen int) string {
	return truncateString(actual, maxLen)
}

// truncateString truncates a string to maxLen, adding "..." if needed.
func truncateString(input string, maxLen int) string {
	// Remove newlines and extra whitespace
	cleaned := strings.ReplaceAll(input, "\n", " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	if len(cleaned) <= maxLen {
		return cleaned
	}

	if maxLen <= minTruncateLen {
		return cleaned[:maxLen]
	}

	return cleaned[:maxLen-minTruncateLen] + "..."
}
