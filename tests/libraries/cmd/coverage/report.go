package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Reporter renders the human coverage report.
type Reporter struct {
	Universe *Universe
	Profile  *Profile
	Out      io.Writer
	OnlyGaps bool
	Source   string
}

// Render prints the report.
func (r *Reporter) Render() {
	total, coveredTotal := len(r.Universe.Branches), len(r.Profile.CoveredIDs())

	r.printf("CHECKLIST BRANCH COVERAGE%38s\n",
		fmt.Sprintf("%d/%d (%.1f%%)", coveredTotal, total, percent(coveredTotal, total)))

	if r.Source != "" {
		r.printf("source: %s\n", r.Source)
	}

	r.printf("evidence: %s\n\n", r.Profile.EvidenceQuality)

	for _, stem := range r.checklistOrder() {
		r.renderChecklist(stem)
	}

	r.renderDiagnostics()
}

// covered answers coverage for a branch id from the profile.
func (r *Reporter) covered(id string) (ProfileBranch, bool) {
	for _, candidate := range r.Profile.Branches {
		if candidate.ID == id {
			return candidate, candidate.State == stateCovered
		}
	}

	return ProfileBranch{}, false
}

// renderChecklist prints one checklist block.
func (r *Reporter) renderChecklist(stem string) {
	prompts := r.promptsOf(stem)
	branchTotal, branchCovered := r.checklistTotals(stem)

	if r.OnlyGaps && branchCovered == branchTotal {
		return
	}

	r.printf("%-13s %d/%d (%.1f%%)\n",
		stem, branchCovered, branchTotal, percent(branchCovered, branchTotal))

	for _, prompt := range prompts {
		line, allCovered := r.promptLine(prompt)
		if r.OnlyGaps && allCovered {
			continue
		}

		r.printf("%s\n", line)
	}

	r.printf("\n")
}

// promptLine renders one prompt's pass/fail(/fix) markers.
func (r *Reporter) promptLine(prompt UniversePrompt) (string, bool) {
	kinds := []BranchKind{KindPass, KindFail}
	if prompt.HasF {
		kinds = append(kinds, KindFix)
	}

	markers := make([]string, 0, len(kinds))
	allCovered := true

	for _, kind := range kinds {
		branch := Branch{Checklist: prompt.Checklist, SectionID: prompt.SectionID,
			Ordinal: prompt.Ordinal, QHash: prompt.QHash, FHash: prompt.FHash, Kind: kind}

		pb, isCovered := r.covered(branch.ID())
		if isCovered {
			markers = append(markers, fmt.Sprintf("%s ✓ %s", kind, strings.Join(pb.Fixtures, ",")))
		} else {
			allCovered = false

			markers = append(markers, string(kind)+" ✗")
		}
	}

	return fmt.Sprintf("  %-22s %-48q %s",
		fmt.Sprintf("%s/q%d@%s", prompt.SectionID, prompt.Ordinal, shortHash(prompt.QHash)),
		prompt.Snippet, strings.Join(markers, "  ")), allCovered
}

// renderDiagnostics prints the uncredited-signal summary.
func (r *Reporter) renderDiagnostics() {
	diags := r.Profile.Diagnostics

	r.printf("protocol-coerced fails (not credited): %d\n", len(diags.ProtocolFails))
	r.printf("non-canonical answers (not credited):  %d\n", len(diags.NonCanonical))
	r.printf("non-shipped prompts exercised:         %d\n", len(diags.NonShipped))
	r.printf("fix candidates (chain incomplete):     %d\n", len(diags.FixCandidates))
	r.printf("unattributed applies:                  %d\n", len(diags.UnattributedApply))
	r.printf("unclassified fails:                    %d\n", len(diags.FailUnclassified))

	for _, line := range diags.HardRunDiagnostics {
		r.printf("HARD: %s\n", line)
	}

	for _, line := range diags.RunDiagnostics {
		r.printf("note: %s\n", line)
	}
}

// checklistOrder returns the distinct checklist stems in prompt order.
func (r *Reporter) checklistOrder() []string {
	seen := map[string]bool{}
	order := make([]string, 0)

	for _, prompt := range r.Universe.Prompts {
		if !seen[prompt.Checklist] {
			seen[prompt.Checklist] = true

			order = append(order, prompt.Checklist)
		}
	}

	sort.Strings(order)

	return order
}

// promptsOf filters the universe prompts of one checklist.
func (r *Reporter) promptsOf(stem string) []UniversePrompt {
	prompts := make([]UniversePrompt, 0)

	for _, prompt := range r.Universe.Prompts {
		if prompt.Checklist == stem {
			prompts = append(prompts, prompt)
		}
	}

	return prompts
}

// checklistTotals counts a checklist's branches and covered branches.
func (r *Reporter) checklistTotals(stem string) (int, int) {
	total, coveredCount := 0, 0

	for _, branch := range r.Universe.Branches {
		if branch.Checklist != stem {
			continue
		}

		total++

		if _, isCovered := r.covered(branch.ID()); isCovered {
			coveredCount++
		}
	}

	return total, coveredCount
}

// printf writes to the report output, ignoring write errors like the
// production table renderer does (output failure is non-fatal).
func (r *Reporter) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.Out, format, args...)
}

// percent guards the zero denominator.
func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(part) / float64(total) * 100 //nolint:mnd // percentage
}
