package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrCannotFix distinguishes an F's obstruction from failed validation and exhausted retries.
var ErrCannotFix = errors.New("cannot fix checklist item")

// ErrCannotFixMalformed refuses an obstruction with no actionable reason or correction.
var ErrCannotFixMalformed = errors.New("cannot_fix requires a reason and what a person must correct")

// CannotFixReport is the optional outcome shared by proposals, clarification replies and applies.
type CannotFixReport struct {
	Reason     string `json:"reason"     yaml:"reason"`
	Correction string `json:"correction" yaml:"correction"`
}

// Refusal returns the obstruction, or nil when the optional outcome is absent.
func (report *CannotFixReport) Refusal() error {
	if report == nil {
		return nil
	}

	if strings.TrimSpace(report.Reason) == "" || strings.TrimSpace(report.Correction) == "" {
		return ErrCannotFixMalformed
	}

	detail := report.Reason + "; correct: " + report.Correction

	return fmt.Errorf("%w: %s", ErrCannotFix, detail)
}
