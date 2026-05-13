package checklist

// Status represents the validation status of a prompt.
type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"

	percentMultiplier = 100 // Multiplier for percentage calculation
)

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// ValidationResult represents the result of a single prompt validation.
type ValidationResult struct {
	SectionPath  string   // e.g., "template/who" or "invest/valuable"
	Question     string   // The question that was asked
	ActualAnswer string   // The actual answer from AI evaluation (pass/fail)
	Context      []string // Per-question context lines from AI (one observation per line)
	Status       Status   // PASS, WARN, FAIL, or SKIP
	Rationale    string   // Why this criterion matters
	FixPrompt    string   // Generated fix prompt when validation fails (optional)
	PromptIndex  int      // Index of the prompt (1-based) for file naming
	Docs         []string // Document keys for this validation (e.g., "prd", "user_roles")
}

// ChecklistReport represents the complete validation report.
type ChecklistReport struct {
	SubjectID    string             // e.g., "4.1" or "INT-011"
	SubjectTitle string             // e.g., "Shared Document Editing"
	Results      []ValidationResult // All validation results
	Summary      ReportSummary      // Aggregated summary
}

// ReportSummary contains aggregated statistics for the report.
type ReportSummary struct {
	TotalPrompts int     // Total number of prompts evaluated
	PassCount    int     // Number of PASS results
	WarnCount    int     // Number of WARN results
	FailCount    int     // Number of FAIL results
	SkipCount    int     // Number of SKIP results
	PassRate     float64 // Percentage of passing prompts (excluding skipped)
}

// CalculateSummary calculates the summary from validation results.
func (r *ChecklistReport) CalculateSummary() {
	r.Summary = ReportSummary{}

	for _, result := range r.Results {
		r.Summary.TotalPrompts++

		switch result.Status {
		case StatusPass:
			r.Summary.PassCount++
		case StatusWarn:
			r.Summary.WarnCount++
		case StatusFail:
			r.Summary.FailCount++
		case StatusSkip:
			r.Summary.SkipCount++
		}
	}

	// Calculate pass rate excluding skipped prompts
	evaluated := r.Summary.TotalPrompts - r.Summary.SkipCount
	if evaluated > 0 {
		r.Summary.PassRate = float64(r.Summary.PassCount) / float64(evaluated) * percentMultiplier
	}
}

// GetOverallStatus returns the overall status based on results.
func (r *ChecklistReport) GetOverallStatus() string {
	if r.Summary.FailCount > 0 {
		return "NEEDS ATTENTION"
	}

	if r.Summary.WarnCount > 0 {
		return "ACCEPTABLE WITH WARNINGS"
	}

	return "PASSED"
}

// AllPassed returns true if all evaluated prompts passed (no failures).
func (r *ChecklistReport) AllPassed() bool {
	return r.Summary.FailCount == 0
}
