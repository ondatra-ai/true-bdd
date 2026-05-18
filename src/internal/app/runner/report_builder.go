package runner

import (
	"sort"

	checklistmodels "bdd-cli/src/internal/domain/models/checklist"
)

// reportBuilder accumulates per-cell ValidationResults into one
// ChecklistReport per subject. Used by the per-command query
// closures so the post-walk render can show the full pass/fail table
// for every item the engine touched.
//
// Results are upserted by (subjectID, promptIndex), so a fix that
// triggers a walker re-walk overwrites stale results with the
// post-fix state — the final render reflects what the engine landed
// on, not the journey.
type reportBuilder struct {
	bySubject map[string]*subjectReport
	order     []string // first-seen ordering of subject IDs
}

type subjectReport struct {
	subjectID    string
	subjectTitle string
	byIndex      map[int]checklistmodels.ValidationResult
}

func newReportBuilder() *reportBuilder {
	return &reportBuilder{bySubject: make(map[string]*subjectReport)}
}

// Add upserts a single cell result.
func (b *reportBuilder) Add(
	subjectID, subjectTitle string,
	result checklistmodels.ValidationResult,
) {
	report, ok := b.bySubject[subjectID]
	if !ok {
		report = &subjectReport{
			subjectID:    subjectID,
			subjectTitle: subjectTitle,
			byIndex:      make(map[int]checklistmodels.ValidationResult),
		}
		b.bySubject[subjectID] = report

		b.order = append(b.order, subjectID)
	}

	report.byIndex[result.PromptIndex] = result
}

// Reports materialises one ChecklistReport per subject, in
// first-seen order, with results sorted by prompt index and
// summary precomputed.
func (b *reportBuilder) Reports() []*checklistmodels.ChecklistReport {
	out := make([]*checklistmodels.ChecklistReport, 0, len(b.order))

	for _, subjectID := range b.order {
		subject := b.bySubject[subjectID]

		indices := make([]int, 0, len(subject.byIndex))
		for idx := range subject.byIndex {
			indices = append(indices, idx)
		}

		sort.Ints(indices)

		results := make([]checklistmodels.ValidationResult, 0, len(indices))
		for _, idx := range indices {
			results = append(results, subject.byIndex[idx])
		}

		report := &checklistmodels.ChecklistReport{
			SubjectID:    subject.subjectID,
			SubjectTitle: subject.subjectTitle,
			Results:      results,
		}
		report.CalculateSummary()

		out = append(out, report)
	}

	return out
}

// RenderAll renders every accumulated report via the supplied renderer.
func (b *reportBuilder) RenderAll(renderer *TableRenderer, fix bool) {
	for _, report := range b.Reports() {
		renderer.RenderReport(report, fix)
	}
}
