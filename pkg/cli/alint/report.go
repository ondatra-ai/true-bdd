package alint

import (
	"encoding/json"
	"fmt"
)

// Status is what a fixing run did about one finding; a checking run leaves it
// empty, having attempted nothing.
type Status string

// The vocabulary alint writes into a fix report.
const (
	// Applied means alint rewrote the file and the finding is gone.
	Applied Status = "applied"
	// Skipped means a fixer existed but declined, e.g. over fix_size_limit.
	Skipped Status = "skipped"
	// Unfixable means the rule declares no fixer; this needs a real edit.
	Unfixable Status = "unfixable"
	// Failed means the fixer ran and did not succeed.
	Failed Status = "failed"
)

// Finding is one rule's complaint about one path. Line and Column are zero
// for a rule whose verdict is about the file rather than a place in it.
type Finding struct {
	RuleID  string
	Level   string
	Path    string
	Message string
	Line    int
	Column  int
	Status  Status
}

func (f Finding) String() string {
	if f.Line == 0 {
		return fmt.Sprintf("%s: %s: %s", f.Path, f.RuleID, f.Message)
	}

	return fmt.Sprintf("%s:%d:%d: %s: %s", f.Path, f.Line, f.Column, f.RuleID, f.Message)
}

// Report is one alint run. Applied, Skipped and Unfixable are zero after a
// check, which attempts no fixes.
type Report struct {
	Applied   int
	Skipped   int
	Unfixable int
	Findings  []Finding
}

// Outstanding is every finding still standing after the run — everything a
// check reports, and everything a fix could not apply.
func (r Report) Outstanding() []Finding {
	var left []Finding

	for _, finding := range r.Findings {
		if finding.Status != Applied {
			left = append(left, finding)
		}
	}

	return left
}

// payload is both subcommands' JSON. They differ only in the key holding the
// findings — `violations` for check, `items` for fix — and in the per-finding
// status, which only a fixing run writes.
type payload struct {
	Summary struct {
		Applied   int `json:"applied"`
		Skipped   int `json:"skipped"`
		Unfixable int `json:"unfixable"`
	} `json:"summary"`
	Results []struct {
		ID         string  `json:"id"`
		Level      string  `json:"level"`
		Violations []entry `json:"violations"`
		Items      []entry `json:"items"`
	} `json:"results"`
}

type entry struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Status  Status `json:"status"`
}

func decode(stdout string) (Report, error) {
	var raw payload

	err := json.Unmarshal([]byte(stdout), &raw)
	if err != nil {
		return Report{}, fmt.Errorf("decoding the %s report: %w", Bin, err)
	}

	report := Report{
		Applied:   raw.Summary.Applied,
		Skipped:   raw.Summary.Skipped,
		Unfixable: raw.Summary.Unfixable,
	}

	for _, result := range raw.Results {
		for _, group := range [][]entry{result.Violations, result.Items} {
			for _, found := range group {
				report.Findings = append(report.Findings, Finding{
					RuleID: result.ID, Level: result.Level, Path: found.Path,
					Message: found.Message, Line: found.Line, Column: found.Column,
					Status: found.Status,
				})
			}
		}
	}

	return report, nil
}
