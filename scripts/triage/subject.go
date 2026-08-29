package triage

import (
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

// bodyLimit caps how much of a subject reaches the turn. It matches the cap
// scripts/clickup renders a ticket body at, so a ticket cannot arrive longer
// than a refresh is allowed to return.
const bodyLimit = 4000

// Subject is one claim about this repository, awaiting judgement.
type Subject struct {
	// ID is the caller's handle — a ticket id, a finding id. Echoed in
	// diagnostics; the turn never sees it as anything but a label.
	ID       string
	Title    string
	Body     string
	File     string
	Line     string
	Origin   string
	Severity string

	// Refresh asks for Body rewritten against HEAD. Set by the callers whose
	// body is a ticket carrying the four headings; false for a review finding,
	// whose body is the reviewer's own prose and whose caller keeps it verbatim.
	Refresh bool
}

// prompt is the whole turn: the rubric, the refresh clause when one is wanted,
// and the subject.
func (s Subject) prompt() string {
	var built strings.Builder

	built.WriteString(rubricPrompt)

	if s.Refresh {
		built.WriteString(refreshPrompt)
	}

	fmt.Fprintf(&built, `
--- BEGIN SUBJECT ---
origin   : %s
file     : %s:%s
severity : %s — whoever raised it said so; score by what you read, not by this
title    : %s

%s
--- END SUBJECT ---
`, orUnknown(s.Origin), orUnknown(s.File), orUnknown(s.Line), orUnknown(s.Severity),
		orUnknown(s.Title), textutil.Truncate(strings.TrimSpace(s.Body), bodyLimit))

	return built.String()
}

// orUnknown keeps an absent field readable: `?` in a labelled block is legible
// where a trailing colon is not.
func orUnknown(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "?"
	}

	return trimmed
}
